package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/queue"
	"github.com/savvyinsight/posthub/internal/storage"
)

// --- Fakes ---

// fakeEnqueuer records enqueue calls and returns a configurable error.
type fakeEnqueuer struct {
	calls []queue.PublishPayload
	err   error
}

func (f *fakeEnqueuer) EnqueuePublish(_ context.Context, payload queue.PublishPayload, _ queue.EnqueueOptions) error {
	f.calls = append(f.calls, payload)
	return f.err
}

// --- Test helpers ---

func newTestRouter(h *PublishHandler, hh *HealthHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/health", hh.HandleHealth)
	r.Post("/publish", h.HandlePublish)
	r.Get("/publish/{id}", h.HandlePublishStatus)
	return r
}

func newTestHandler(enqueueErr error) (*PublishHandler, *fakeEnqueuer) {
	ms := storage.NewMemoryStorage()
	enqueuer := &fakeEnqueuer{err: enqueueErr}
	return &PublishHandler{
		ContentStore: ms.Content,
		TaskStore:    ms.Tasks,
		Enqueuer:     enqueuer,
	}, enqueuer
}

// --- Tests ---

func TestHandleHealth(t *testing.T) {
	hh := &HealthHandler{Version: "1.0.0"}
	h, _ := newTestHandler(nil)
	r := newTestRouter(h, hh)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", resp.Version)
	}
}

func TestHandlePublish_Success(t *testing.T) {
	h, fake := newTestHandler(nil)
	hh := &HealthHandler{Version: "test"}
	r := newTestRouter(h, hh)

	body := publishRequest{
		Title:     "Test Post",
		Body:      "Hello world",
		Tags:      []string{"golang", "api"},
		Platforms: []string{"zhihu", "bilibili"},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contracts.PublishBatchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ContentID == "" {
		t.Error("expected content_id to be set")
	}
	if len(resp.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(resp.Tasks))
	}

	// Verify platforms are present.
	platforms := map[string]bool{}
	for _, task := range resp.Tasks {
		platforms[task.Platform] = true
		if task.TaskID == "" {
			t.Errorf("expected task_id for platform %s", task.Platform)
		}
		if task.Status != contracts.PublishTaskStatusPending {
			t.Errorf("expected status 'pending', got %q", task.Status)
		}
	}
	if !platforms["zhihu"] || !platforms["bilibili"] {
		t.Errorf("expected both platforms, got %v", platforms)
	}

	// Verify enqueue was called for each platform.
	if len(fake.calls) != 2 {
		t.Fatalf("expected 2 enqueue calls, got %d", len(fake.calls))
	}
}

func TestHandlePublish_ValidationErrors(t *testing.T) {
	tests := []struct {
		name       string
		req        publishRequest
		wantFields []string
	}{
		{
			name:       "missing title",
			req:        publishRequest{Body: "body", Platforms: []string{"zhihu"}},
			wantFields: []string{"title"},
		},
		{
			name:       "missing body",
			req:        publishRequest{Title: "title", Platforms: []string{"zhihu"}},
			wantFields: []string{"body"},
		},
		{
			name:       "missing platforms",
			req:        publishRequest{Title: "title", Body: "body"},
			wantFields: []string{"platforms"},
		},
		{
			name:       "all empty",
			req:        publishRequest{},
			wantFields: []string{"title", "body", "platforms"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(nil)
			hh := &HealthHandler{Version: "test"}
			r := newTestRouter(h, hh)

			payload, _ := json.Marshal(tt.req)
			req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp contracts.AppError
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode error: %v", err)
			}
			if resp.Code != contracts.ErrCodeValidation {
				t.Errorf("expected error code 'validation_error', got %q", resp.Code)
			}

			gotFields := map[string]bool{}
			for _, d := range resp.Details {
				gotFields[d.Field] = true
			}
			for _, field := range tt.wantFields {
				if !gotFields[field] {
					t.Errorf("expected error detail for field %q, got details %v", resp.Details, tt.wantFields)
				}
			}
		})
	}
}

func TestHandlePublish_InvalidJSON(t *testing.T) {
	h, _ := newTestHandler(nil)
	hh := &HealthHandler{Version: "test"}
	r := newTestRouter(h, hh)

	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandlePublishStatus_Success(t *testing.T) {
	ms := storage.NewMemoryStorage()
	enqueuer := &fakeEnqueuer{}
	h := &PublishHandler{
		ContentStore: ms.Content,
		TaskStore:    ms.Tasks,
		Enqueuer:     enqueuer,
	}
	hh := &HealthHandler{Version: "test"}
	r := newTestRouter(h, hh)

	// Seed: create content and a task.
	ctx := context.Background()
	content, err := ms.Content.CreateContent(ctx, "Title", "Body", nil)
	if err != nil {
		t.Fatalf("seed content: %v", err)
	}
	task, err := ms.Tasks.CreateTask(ctx, content.ID, "zhihu")
	if err != nil {
		t.Fatalf("seed task: %v", err)
	}
	_ = task

	req := httptest.NewRequest(http.MethodGet, "/publish/"+content.ID, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contracts.PublishStatus
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ContentID != content.ID {
		t.Errorf("expected content_id %q, got %q", content.ID, resp.ContentID)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
	if resp.Tasks[0].Platform != "zhihu" {
		t.Errorf("expected platform 'zhihu', got %q", resp.Tasks[0].Platform)
	}
}

func TestHandlePublishStatus_NotFound(t *testing.T) {
	h, _ := newTestHandler(nil)
	hh := &HealthHandler{Version: "test"}
	r := newTestRouter(h, hh)

	req := httptest.NewRequest(http.MethodGet, "/publish/nonexistent", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contracts.AppError
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error: %v", err)
	}
	if resp.Code != contracts.ErrCodeNotFound {
		t.Errorf("expected error code 'not_found', got %q", resp.Code)
	}
}

func TestHandlePublish_EnqueueError(t *testing.T) {
	// Enqueue fails but task creation succeeds — should still return 202
	// because tasks are created (just not enqueued).
	h, _ := newTestHandler(errors.New("queue unavailable"))
	hh := &HealthHandler{Version: "test"}
	r := newTestRouter(h, hh)

	body := publishRequest{
		Title:     "Test",
		Body:      "Body",
		Platforms: []string{"zhihu"},
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/publish", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp contracts.PublishBatchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(resp.Tasks))
	}
}

func TestValidatePublishRequest_TitleTooLong(t *testing.T) {
	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	req := &publishRequest{
		Title:     string(longTitle),
		Body:      "body",
		Platforms: []string{"zhihu"},
	}

	err := validatePublishRequest(req)
	if err == nil {
		t.Fatal("expected validation error for long title")
	}

	found := false
	for _, d := range err.Details {
		if d.Field == "title" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected title error detail, got %v", err.Details)
	}
}
