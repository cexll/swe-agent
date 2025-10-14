package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"

	"github.com/cexll/swe/internal/taskstore"
)

func newTemplates(listTpl, detailTpl string, t *testing.T) *template.Template {
	t.Helper()
	tmpl := template.Must(template.New("list.html").Parse(listTpl))
	template.Must(tmpl.New("detail.html").Parse(detailTpl))
	return tmpl
}

func TestNewHandler(t *testing.T) {
	store := taskstore.NewStore()
	tempDir := t.TempDir()
	templatesDir := filepath.Join(tempDir, "templates")
	if err := os.MkdirAll(templatesDir, 0o755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "list.html"), []byte("{{define \"list.html\"}}ok{{end}}"), 0o644); err != nil {
		t.Fatalf("failed to write list template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(templatesDir, "detail.html"), []byte("{{define \"detail.html\"}}{{.Task.ID}}{{end}}"), 0o644); err != nil {
		t.Fatalf("failed to write detail template: %v", err)
	}

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(originalWD)
	})

	handler, err := NewHandler(store)
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}
	if handler.store != store {
		t.Fatalf("handler store mismatch")
	}
	if handler.templates == nil {
		t.Fatalf("handler templates not initialized")
	}
}

func TestNewHandler_TemplateParseError(t *testing.T) {
	store := taskstore.NewStore()
	tempDir := t.TempDir()

	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(originalWD)
	})

	if _, err := NewHandler(store); err == nil {
		t.Fatal("expected error when templates directory missing")
	}
}

func TestHandler_ListTasks_NoStore(t *testing.T) {
	handler := &Handler{
		store:     nil,
		templates: newTemplates("ok", "{{.Task.ID}}", t),
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()

	handler.ListTasks(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rr.Body.String(), "task store unavailable") {
		t.Fatalf("body = %q, want error message", rr.Body.String())
	}
}

func TestHandler_ListTasks_TemplateError(t *testing.T) {
	store := taskstore.NewStore()
	handler := &Handler{
		store:     store,
		templates: newTemplates("{{index .Tasks 0}}", "{{.Task.ID}}", t),
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()

	handler.ListTasks(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestHandler_ListTasks_Success(t *testing.T) {
	store := taskstore.NewStore()
	store.Create(&taskstore.Task{ID: "task-123", Title: "demo"})

	handler := &Handler{
		store:     store,
		templates: newTemplates("{{range .Tasks}}{{.ID}}{{end}}", "{{.Task.ID}}", t),
	}

	req := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	rr := httptest.NewRecorder()

	handler.ListTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "task-123") {
		t.Fatalf("body = %q, want task id", rr.Body.String())
	}
}

func TestHandler_TaskDetail_NoStore(t *testing.T) {
	handler := &Handler{
		templates: newTemplates("ok", "{{.Task.ID}}", t),
	}

	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/tasks/task-123", nil), map[string]string{"id": "task-123"})
	rr := httptest.NewRecorder()

	handler.TaskDetail(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

func TestHandler_TaskDetail_NotFound(t *testing.T) {
	store := taskstore.NewStore()
	handler := &Handler{
		store:     store,
		templates: newTemplates("ok", "{{.Task.ID}}", t),
	}

	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/tasks/missing", nil), map[string]string{"id": "missing"})
	rr := httptest.NewRecorder()

	handler.TaskDetail(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandler_TaskDetail_Success(t *testing.T) {
	store := taskstore.NewStore()
	store.Create(&taskstore.Task{ID: "task-123", Title: "demo"})

	handler := &Handler{
		store:     store,
		templates: newTemplates("ok", "{{.Task.ID}}", t),
	}

	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/tasks/task-123", nil), map[string]string{"id": "task-123"})
	rr := httptest.NewRecorder()

	handler.TaskDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if rr.Body.String() != "task-123" {
		t.Fatalf("body = %q, want task-123", rr.Body.String())
	}
}
