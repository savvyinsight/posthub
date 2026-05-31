// Package web provides HTTP handlers for the demo UI.
//
// Handlers serve server-rendered HTML with HTMX for dynamic updates.
// This is a minimal demo UI — not production frontend code.
package web

import (
	"embed"
	"html/template"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/savvyinsight/posthub/internal/contracts"
	"github.com/savvyinsight/posthub/internal/storage"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Handler serves the web UI.
type Handler struct {
	ContentStore storage.ContentStore
	TaskStore    storage.PublishTaskStore
	templates    *template.Template
}

// NewHandler creates a new web handler with parsed templates.
func NewHandler(cs storage.ContentStore, ts storage.PublishTaskStore) *Handler {
	tmpl := template.Must(template.ParseFS(templateFS, "templates/*.html"))
	return &Handler{
		ContentStore: cs,
		TaskStore:    ts,
		templates:    tmpl,
	}
}

// Routes registers the web UI routes on the given router.
func (h *Handler) Routes(r chi.Router) {
	// Serve static files
	r.Handle("/static/*", http.FileServer(http.FS(staticFS)))

	// Pages
	r.Get("/", h.HandleIndex)
	r.Post("/publish", h.HandlePublish)
	r.Get("/status/{id}", h.HandleStatus)
	r.Get("/status/{id}/poll", h.HandleStatusPoll)
	r.Get("/health", h.HandleHealth)
}

// pageData holds data passed to templates.
type pageData struct {
	ContentID string
	Tasks     []contracts.TaskStatus
	Status    contracts.ContentStatus
	Error     string
}

// HandleIndex renders the main page.
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	h.templates.ExecuteTemplate(w, "index.html", nil)
}

// HandlePublish creates a publish job via HTMX form submission.
func (h *Handler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	title := r.FormValue("title")
	body := r.FormValue("body")
	platform := r.FormValue("platform")

	if title == "" || body == "" || platform == "" {
		h.templates.ExecuteTemplate(w, "error.html", pageData{
			Error: "All fields are required",
		})
		return
	}

	// Create content
	content, err := h.ContentStore.CreateContent(ctx, title, body, nil)
	if err != nil {
		h.templates.ExecuteTemplate(w, "error.html", pageData{
			Error: "Failed to create content",
		})
		return
	}

	// Transition status
	_ = h.ContentStore.UpdateContent(ctx, content.ID, contracts.ContentStatusReady, 1)
	_ = h.ContentStore.UpdateContent(ctx, content.ID, contracts.ContentStatusPublishing, 2)

	// Create task
	task, err := h.TaskStore.CreateTask(ctx, content.ID, platform)
	if err != nil {
		h.templates.ExecuteTemplate(w, "error.html", pageData{
			Error: "Failed to create publish task",
		})
		return
	}

	// Return status partial with polling
	h.templates.ExecuteTemplate(w, "status.html", pageData{
		ContentID: content.ID,
		Tasks: []contracts.TaskStatus{
			{
				TaskID:    task.ID,
				Platform:  task.Platform,
				Status:    task.Status,
				CreatedAt: task.CreatedAt,
				UpdatedAt: task.UpdatedAt,
			},
		},
		Status: content.Status,
	})
}

// HandleStatus renders the full status page.
func (h *Handler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contentID := chi.URLParam(r, "id")

	content, err := h.ContentStore.GetContent(ctx, contentID)
	if err != nil {
		h.templates.ExecuteTemplate(w, "error.html", pageData{
			Error: "Content not found",
		})
		return
	}

	taskList, _ := h.TaskStore.GetTasksByContent(ctx, contentID)
	tasks := make([]contracts.TaskStatus, 0, len(taskList))
	for _, t := range taskList {
		tasks = append(tasks, contracts.TaskStatus{
			TaskID:    t.ID,
			Platform:  t.Platform,
			Status:    t.Status,
			Error:     t.Error,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}

	h.templates.ExecuteTemplate(w, "status.html", pageData{
		ContentID: content.ID,
		Tasks:     tasks,
		Status:    content.Status,
	})
}

// HandleStatusPoll returns status partial for HTMX polling.
func (h *Handler) HandleStatusPoll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contentID := chi.URLParam(r, "id")

	content, err := h.ContentStore.GetContent(ctx, contentID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	taskList, _ := h.TaskStore.GetTasksByContent(ctx, contentID)
	tasks := make([]contracts.TaskStatus, 0, len(taskList))
	for _, t := range taskList {
		tasks = append(tasks, contracts.TaskStatus{
			TaskID:    t.ID,
			Platform:  t.Platform,
			Status:    t.Status,
			Error:     t.Error,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}

	// Check if all tasks are terminal — stop polling
	allTerminal := true
	for _, t := range tasks {
		if t.Status.IsActive() {
			allTerminal = false
			break
		}
	}

	data := pageData{
		ContentID: content.ID,
		Tasks:     tasks,
		Status:    content.Status,
	}

	if allTerminal {
		h.templates.ExecuteTemplate(w, "status-final.html", data)
	} else {
		h.templates.ExecuteTemplate(w, "status-poll.html", data)
	}
}

// HandleHealth returns a simple health check page.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html><html><body><h1>OK</h1></body></html>`))
}
