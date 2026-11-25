package websocket

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Srinu0725/algotrace/internal/jobs"
	ws "github.com/gorilla/websocket"
)

func newTestServer(
	t *testing.T,
) (*jobs.Manager, *httptest.Server) {

	t.Helper()

	manager := jobs.NewManager(10)

	server := httptest.NewServer(
		NewHandler(manager),
	)

	t.Cleanup(func() {

		server.Close()

		ctx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)

		defer cancel()

		if err := manager.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})

	return manager, server
}

func submitJob(
	t *testing.T,
	manager *jobs.Manager,
	id string,
) {

	t.Helper()

	err := manager.Submit(jobs.Job{
		ID:   id,
		Code: "print(30)",
		Mode: jobs.ModeRun,
	})

	if err != nil {
		t.Fatal(err)
	}
}

func websocketURL(
	server *httptest.Server,
	id string,
) string {

	return "ws" +
		strings.TrimPrefix(server.URL, "http") +
		"/ws/jobs/" + id
}

func connect(
	t *testing.T,
	server *httptest.Server,
	id string,
) *ws.Conn {

	t.Helper()

	dialer := ws.Dialer{
		HandshakeTimeout: 3 * time.Second,
	}

	conn, response, err := dialer.Dial(
		websocketURL(server, id),
		nil,
	)

	if err != nil {

		if response != nil {
			response.Body.Close()
		}

		t.Fatal(err)
	}

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

func readEvent(
	t *testing.T,
	conn *ws.Conn,
) jobs.JobEvent {

	t.Helper()

	var event jobs.JobEvent

	if err := conn.SetReadDeadline(
		time.Now().Add(3 * time.Second),
	); err != nil {
		t.Fatal(err)
	}

	if err := conn.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}

	return event
}

func TestWebSocketInitialSnapshot(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	conn := connect(t, server, "job-1")

	event := readEvent(t, conn)

	if event.JobID != "job-1" {
		t.Fatal("incorrect job ID")
	}

	if event.Status != jobs.StatusQueued {
		t.Fatalf("unexpected status: %s", event.Status)
	}
}

func TestWebSocketUnknownJob(t *testing.T) {

	_, server := newTestServer(t)

	conn, response, err := ws.DefaultDialer.Dial(
		websocketURL(server, "missing"),
		nil,
	)

	if conn != nil {
		conn.Close()
	}

	if response != nil {
		defer response.Body.Close()
	}

	if err == nil {
		t.Fatal("expected handshake failure")
	}

	if response == nil ||
		response.StatusCode != http.StatusNotFound {

		t.Fatal("expected HTTP 404")
	}
}

func TestWebSocketJobLifecycle(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	conn := connect(t, server, "job-1")

	initial := readEvent(t, conn)

	if initial.Status != jobs.StatusQueued {
		t.Fatal("expected queued")
	}

	release := make(chan struct{})

	var once sync.Once

	unblock := func() {
		once.Do(func() {
			close(release)
		})
	}

	defer unblock()

	err := manager.StartWorker(
		func(string) (jobs.ExecutionResult, error) {

			<-release

			return jobs.ExecutionResult{
				Stdout:     "30\n",
				ExitCode:   0,
				DurationMS: 100,
			}, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	running := readEvent(t, conn)

	if running.Status != jobs.StatusRunning {
		t.Fatal("expected running")
	}

	unblock()

	completed := readEvent(t, conn)

	if completed.Status != jobs.StatusCompleted {
		t.Fatal("expected completed")
	}

	_, _, err = conn.ReadMessage()

	if !ws.IsCloseError(err, ws.CloseNormalClosure) {
		t.Fatalf("expected normal closure, got %v", err)
	}
}

func TestWebSocketMultipleClients(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	const count = 5

	connections := make([]*ws.Conn, 0, count)

	for i := 0; i < count; i++ {

		conn := connect(t, server, "job-1")

		readEvent(t, conn)

		connections = append(connections, conn)
	}

	err := manager.StartWorker(
		func(string) (jobs.ExecutionResult, error) {

			return jobs.ExecutionResult{
				ExitCode: 0,
			}, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	for _, conn := range connections {

		for {

			event := readEvent(t, conn)

			if jobs.IsTerminalStatus(event.Status) {

				if event.Status != jobs.StatusCompleted {
					t.Fatal("execution failed")
				}

				break
			}
		}
	}
}

func TestWebSocketCompletedJob(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	err := manager.StartWorker(
		func(string) (jobs.ExecutionResult, error) {

			return jobs.ExecutionResult{
				ExitCode: 0,
			}, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)

	for {

		job, _ := manager.Get("job-1")

		if job.Status == jobs.StatusCompleted {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("job did not complete")
		}

		time.Sleep(10 * time.Millisecond)
	}

	conn := connect(t, server, "job-1")

	event := readEvent(t, conn)

	if event.Status != jobs.StatusCompleted {
		t.Fatal("expected completed snapshot")
	}

	_, _, err = conn.ReadMessage()

	if !ws.IsCloseError(err, ws.CloseNormalClosure) {
		t.Fatalf("expected normal closure, got %v", err)
	}
}

func TestWebSocketRejectsHTTP(t *testing.T) {

	_, server := newTestServer(t)

	response, err := http.Get(
		server.URL + "/ws/jobs/job-1",
	)

	if err != nil {
		t.Fatal(err)
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusUpgradeRequired {
		t.Fatalf(
			"unexpected status: %d",
			response.StatusCode,
		)
	}
}

func TestWebSocketRejectsCrossOrigin(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	headers := http.Header{}

	headers.Set("Origin", "https://example.com")

	conn, response, err := ws.DefaultDialer.Dial(
		websocketURL(server, "job-1"),
		headers,
	)

	if conn != nil {
		conn.Close()
	}

	if response != nil {
		defer response.Body.Close()
	}

	if err == nil {
		t.Fatal("expected origin rejection")
	}

	if response == nil ||
		response.StatusCode != http.StatusForbidden {

		t.Fatal("expected HTTP 403")
	}
}

func TestWebSocketClientDisconnect(t *testing.T) {

	manager, server := newTestServer(t)

	submitJob(t, manager, "job-1")

	conn := connect(t, server, "job-1")

	readEvent(t, conn)

	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}

	err := manager.StartWorker(
		func(string) (jobs.ExecutionResult, error) {

			return jobs.ExecutionResult{
				ExitCode: 0,
			}, nil
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)

	for {

		job, _ := manager.Get("job-1")

		if job.Status == jobs.StatusCompleted {
			return
		}

		if time.Now().After(deadline) {
			t.Fatal("execution blocked after disconnect")
		}

		time.Sleep(10 * time.Millisecond)
	}
}
