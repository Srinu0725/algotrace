package websocket

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Srinu0725/algotrace/internal/jobs"
	ws "github.com/gorilla/websocket"
)

const (
	writeWait      = 5 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 1024
)

type Handler struct {
	manager  *jobs.Manager
	upgrader ws.Upgrader
}

func NewHandler(manager *jobs.Manager) *Handler {
	return &Handler{
		manager: manager,
		upgrader: ws.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,

			// Gorilla's default same-origin policy
			// is retained for now.
		},
	}
}

func (h *Handler) ServeHTTP(
	w http.ResponseWriter,
	r *http.Request,
) {

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)

		http.Error(
			w,
			"method not allowed",
			http.StatusMethodNotAllowed,
		)

		return
	}

	jobID := strings.TrimPrefix(
		r.URL.Path,
		"/ws/jobs/",
	)

	if jobID == "" ||
		jobID == r.URL.Path ||
		strings.Contains(jobID, "/") {

		http.Error(
			w,
			"invalid job ID",
			http.StatusBadRequest,
		)

		return
	}

	if !ws.IsWebSocketUpgrade(r) {
		http.Error(
			w,
			"WebSocket upgrade required",
			http.StatusUpgradeRequired,
		)

		return
	}

	events, cancel, err := h.manager.Subscribe(jobID)

	if err != nil {

		switch {

		case errors.Is(err, jobs.ErrJobNotFound):

			http.Error(
				w,
				"job not found",
				http.StatusNotFound,
			)

		case errors.Is(err, jobs.ErrSubscriberLimit):

			http.Error(
				w,
				"subscriber limit reached",
				http.StatusTooManyRequests,
			)

		default:

			http.Error(
				w,
				"service unavailable",
				http.StatusServiceUnavailable,
			)
		}

		return
	}

	defer cancel()

	conn, err := h.upgrader.Upgrade(w, r, nil)

	if err != nil {
		return
	}

	defer conn.Close()

	conn.SetReadLimit(maxMessageSize)

	if err := conn.SetReadDeadline(
		time.Now().Add(pongWait),
	); err != nil {
		return
	}

	conn.SetPongHandler(func(string) error {

		return conn.SetReadDeadline(
			time.Now().Add(pongWait),
		)
	})

	disconnected := make(chan struct{})

	// One reader goroutine detects disconnects
	// and processes WebSocket control frames.
	go func() {

		defer close(disconnected)

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)

	defer ticker.Stop()

	// This goroutine is the sole application writer.
	for {

		select {

		case <-disconnected:
			return

		case <-r.Context().Done():
			return

		case event, ok := <-events:

			if !ok {

				_ = conn.WriteControl(
					ws.CloseMessage,
					ws.FormatCloseMessage(
						ws.CloseNormalClosure,
						"subscription finished",
					),
					time.Now().Add(writeWait),
				)

				return
			}

			if err := conn.SetWriteDeadline(
				time.Now().Add(writeWait),
			); err != nil {
				return
			}

			if err := conn.WriteJSON(event); err != nil {
				return
			}

			if jobs.IsTerminalStatus(event.Status) {

				_ = conn.WriteControl(
					ws.CloseMessage,
					ws.FormatCloseMessage(
						ws.CloseNormalClosure,
						"execution finished",
					),
					time.Now().Add(writeWait),
				)

				return
			}

		case <-ticker.C:

			if err := conn.WriteControl(
				ws.PingMessage,
				nil,
				time.Now().Add(writeWait),
			); err != nil {
				return
			}
		}
	}
}
