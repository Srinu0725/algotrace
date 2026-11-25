package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Srinu0725/algotrace/internal/jobs"
)

func TestHealthEndpoint(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "AlgoTrace is running") {
		t.Fatalf("unexpected response: %s", rec.Body.String())
	}
}

func TestHealthEndpointRejectsPOST(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}

	if rec.Header().Get("Allow") != http.MethodGet {
		t.Fatal("expected Allow header to be GET")
	}
}

func TestExecuteEndpointAcceptsValidRequest(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	body := `{"code":"print(10 + 20)"}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/execute",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var response ExecuteResponse

	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	if response.JobID == "" {
		t.Fatal("expected a job ID")
	}

	if response.Status != "queued" {
		t.Fatalf("expected queued status, got %q", response.Status)
	}

	job, ok := manager.Get(response.JobID)

	if !ok {
		t.Fatal("submitted job was not stored")
	}

	if job.Code != "print(10 + 20)" {
		t.Fatalf("unexpected stored code: %q", job.Code)
	}
}

func TestExecuteEndpointRejectsGET(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(http.MethodGet, "/execute", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405, got %d", rec.Code)
	}
}

func TestExecuteEndpointRejectsEmptyCode(t *testing.T) {
	tests := []string{
		`{"code":""}`,
		`{"code":"   "}`,
	}

	for _, body := range tests {
		t.Run(body, func(t *testing.T) {
			manager := jobs.NewManager(10)
			handler := newHandler(manager)

			req := httptest.NewRequest(
				http.MethodPost,
				"/execute",
				strings.NewReader(body),
			)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", rec.Code)
			}
		})
	}
}

func TestExecuteEndpointRejectsInvalidJSON(t *testing.T) {
	tests := []string{
		`{"code":`,
		`not-json`,
		``,
	}

	for _, body := range tests {
		t.Run(body, func(t *testing.T) {
			manager := jobs.NewManager(10)
			handler := newHandler(manager)

			req := httptest.NewRequest(
				http.MethodPost,
				"/execute",
				strings.NewReader(body),
			)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d", rec.Code)
			}
		})
	}
}

func TestExecuteEndpointRejectsUnknownFields(t *testing.T) {
	body := `{"code":"print(10)","admin":true}`

	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(
		http.MethodPost,
		"/execute",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestExecuteEndpointRejectsMultipleJSONValues(t *testing.T) {
	body := `{"code":"print(10)"} {"code":"print(20)"}`

	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(
		http.MethodPost,
		"/execute",
		strings.NewReader(body),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestExecuteEndpointRejectsOversizedBody(t *testing.T) {
	largeCode := strings.Repeat("A", maxRequestBody)

	body, err := json.Marshal(ExecuteRequest{
		Code: largeCode,
	})

	if err != nil {
		t.Fatal(err)
	}

	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(
		http.MethodPost,
		"/execute",
		bytes.NewReader(body),
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", rec.Code)
	}
}

func TestGetJobEndpointReturnsNotFound(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(
		http.MethodGet,
		"/jobs/non-existent",
		nil,
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestGetJobEndpointRejectsInvalidID(t *testing.T) {
	manager := jobs.NewManager(10)
	handler := newHandler(manager)

	req := httptest.NewRequest(
		http.MethodGet,
		"/jobs/",
		nil,
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}
