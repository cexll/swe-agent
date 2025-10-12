package web

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/cexll/swe/internal/taskstore"
)

type Handler struct {
	store     *taskstore.Store
	templates *template.Template
}

func NewHandler(store *taskstore.Store) (*Handler, error) {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Handler{
		store:     store,
		templates: tmpl,
	}, nil
}

func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "task store unavailable", http.StatusServiceUnavailable)
		return
	}
	tasks := h.store.List()
	if err := h.templates.ExecuteTemplate(w, "list.html", map[string]interface{}{
		"Tasks": tasks,
	}); err != nil {
		http.Error(w, "template rendering error", http.StatusInternalServerError)
	}
}

func (h *Handler) TaskDetail(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		http.Error(w, "task store unavailable", http.StatusServiceUnavailable)
		return
	}
	vars := mux.Vars(r)
	taskID := vars["id"]

	task, ok := h.store.Get(taskID)
	if !ok {
		http.NotFound(w, r)
		return
	}

	if err := h.templates.ExecuteTemplate(w, "detail.html", map[string]interface{}{
		"Task": task,
	}); err != nil {
		http.Error(w, "template rendering error", http.StatusInternalServerError)
	}
}
