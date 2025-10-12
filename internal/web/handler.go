package web

import (
	"embed"
	"html/template"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"github.com/stellarlink/pilot-swe/internal/store"
)

//go:embed templates/*
var templatesFS embed.FS

// Handler handles web UI requests
type Handler struct {
	store     *store.TaskStore
	templates *template.Template
}

// NewHandler creates a new web handler
func NewHandler(taskStore *store.TaskStore) (*Handler, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Handler{
		store:     taskStore,
		templates: tmpl,
	}, nil
}

// RegisterRoutes registers web UI routes
func (h *Handler) RegisterRoutes(r *mux.Router) {
	r.HandleFunc("/", h.handleTaskList).Methods("GET")
	r.HandleFunc("/task/{id}", h.handleTaskDetail).Methods("GET")
}

// handleTaskList renders the task list page
func (h *Handler) handleTaskList(w http.ResponseWriter, r *http.Request) {
	tasks := h.store.List()

	data := struct {
		Tasks []*store.Task
	}{
		Tasks: tasks,
	}

	if err := h.templates.ExecuteTemplate(w, "task_list.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// handleTaskDetail renders the task detail page
func (h *Handler) handleTaskDetail(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, err := h.store.Get(taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	data := struct {
		Task *store.Task
	}{
		Task: task,
	}

	if err := h.templates.ExecuteTemplate(w, "task_detail.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Helper functions for templates
func statusColor(status store.TaskStatus) string {
	switch status {
	case store.StatusPending:
		return "#6c757d"
	case store.StatusRunning:
		return "#0d6efd"
	case store.StatusCompleted:
		return "#198754"
	case store.StatusFailed:
		return "#dc3545"
	default:
		return "#6c757d"
	}
}

func statusIcon(status store.TaskStatus) string {
	switch status {
	case store.StatusPending:
		return "○"
	case store.StatusRunning:
		return "⟳"
	case store.StatusCompleted:
		return "✓"
	case store.StatusFailed:
		return "✗"
	default:
		return "○"
	}
}

func logLevelColor(level string) string {
	switch strings.ToLower(level) {
	case "error":
		return "#dc3545"
	case "success":
		return "#198754"
	case "info":
		return "#0d6efd"
	default:
		return "#6c757d"
	}
}

func init() {
	// Register template functions
	template.FuncMap{
		"statusColor":    statusColor,
		"statusIcon":     statusIcon,
		"logLevelColor":  logLevelColor,
	}
}