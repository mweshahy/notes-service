package httpadapter_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/notes/internal/domain"
	"example.com/notes/internal/service"
	"example.com/notes/internal/storage"
	httpadapter "example.com/notes/internal/transport/http"
)

type fakeService struct {
	createNote                                       *domain.Note
	getNote                                          *domain.Note
	listNotes                                        []domain.Note
	createErr, getErr, listErr, updateErr, deleteErr error
	updated                                          *domain.Note
	deletedID                                        string
}

func (f *fakeService) CreateNote(context.Context, string, string) (*domain.Note, error) {
	return f.createNote, f.createErr
}
func (f *fakeService) GetNote(context.Context, string) (*domain.Note, error) {
	return f.getNote, f.getErr
}
func (f *fakeService) List(context.Context, string) ([]domain.Note, error) {
	return f.listNotes, f.listErr
}
func (f *fakeService) Update(_ context.Context, n *domain.Note) error {
	f.updated = n
	return f.updateErr
}
func (f *fakeService) Delete(_ context.Context, id string) error {
	f.deletedID = id
	return f.deleteErr
}

func newTestHandler(s *fakeService) *httpadapter.Handler {
	return httpadapter.NewHandler(s, slog.New(slog.NewTextHandler(io.Discard, nil)))
}
func serve(handler http.HandlerFunc, method, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	handler(w, r)
	return w
}

func TestHandleCreateNote(t *testing.T) {
	internalErr := errors.New("db unavailable")
	tests := []struct {
		name, body string
		note       *domain.Note
		err        error
		want       int
	}{
		{name: "created", body: `{"title":"T","content":"C"}`, note: &domain.Note{ID: "id"}, want: http.StatusCreated},
		{name: "invalid input", body: `{"title":"T","content":"C"}`, err: domain.ErrInvalidInput, want: http.StatusBadRequest},
		{name: "service error", body: `{"title":"T","content":"C"}`, err: internalErr, want: http.StatusInternalServerError},
		{name: "malformed JSON", body: `{`, want: http.StatusBadRequest},
		{name: "unknown field", body: `{"title":"T","content":"C","other":1}`, want: http.StatusBadRequest},
		{name: "multiple values", body: `{"title":"T","content":"C"} {}`, want: http.StatusBadRequest},
		{name: "oversized body", body: `{"title":"` + strings.Repeat("x", 1<<20) + `","content":"C"}`, want: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := serve(newTestHandler(&fakeService{createNote: tt.note, createErr: tt.err}).HandleCreateNote, http.MethodPost, "/api/v1/notes", tt.body)
			if w.Code != tt.want {
				t.Fatalf("status = %d, want %d; body %s", w.Code, tt.want, w.Body.String())
			}
		})
	}
}

func TestHandleListNotes(t *testing.T) {
	errFailure := errors.New("search failed")
	for _, tc := range []struct {
		name  string
		notes []domain.Note
		err   error
		want  int
	}{{"success", []domain.Note{}, nil, 200}, {"service error", nil, errFailure, 500}} {
		t.Run(tc.name, func(t *testing.T) {
			w := serve(newTestHandler(&fakeService{listNotes: tc.notes, listErr: tc.err}).HandleListNotes, "GET", "/api/v1/notes?q=word", "")
			if w.Code != tc.want {
				t.Fatalf("status = %d", w.Code)
			}
		})
	}
}

func TestHandleGetNote(t *testing.T) {
	for _, tc := range []struct {
		name string
		note *domain.Note
		err  error
		want int
	}{{"success", &domain.Note{ID: "id"}, nil, 200}, {"not found", nil, domain.ErrNoteNotFound, 404}, {"service error", nil, errors.New("failure"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestHandler(&fakeService{getNote: tc.note, getErr: tc.err})
			mux := http.NewServeMux()
			mux.HandleFunc("GET /api/v1/notes/{id}", h.HandleGetNote)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/notes/id", nil))
			if w.Code != tc.want {
				t.Fatalf("status = %d", w.Code)
			}
		})
	}
}

func TestHandleUpdateNote(t *testing.T) {
	base := &domain.Note{ID: "id", Title: "Old", Content: "Body", CreatedAt: time.Now().Add(-time.Hour)}
	for _, tc := range []struct {
		name, body        string
		getErr, updateErr error
		want              int
	}{{"success", `{"title":"New","content":"Body"}`, nil, nil, 200}, {"invalid JSON", `bad`, nil, nil, 400}, {"get not found", `{"title":"New","content":"Body"}`, domain.ErrNoteNotFound, nil, 404}, {"get error", `{"title":"New","content":"Body"}`, errors.New("get"), nil, 500}, {"invalid input", `{"title":" ","content":"Body"}`, nil, nil, 400}, {"update not found", `{"title":"New","content":"Body"}`, nil, domain.ErrNoteNotFound, 404}, {"update error", `{"title":"New","content":"Body"}`, nil, errors.New("update"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{getNote: &domain.Note{ID: base.ID, Title: base.Title, Content: base.Content, CreatedAt: base.CreatedAt}, getErr: tc.getErr, updateErr: tc.updateErr}
			h := newTestHandler(svc)
			mux := http.NewServeMux()
			mux.HandleFunc("PUT /api/v1/notes/{id}", h.HandleUpdateNote)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("PUT", "/api/v1/notes/id", strings.NewReader(tc.body)))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d; %s", w.Code, tc.want, w.Body.String())
			}
			if tc.name == "success" && (svc.updated == nil || svc.updated.Title != "New" || !svc.updated.UpdatedAt.After(base.CreatedAt)) {
				t.Fatalf("updated note = %#v", svc.updated)
			}
		})
	}
}

func TestHandleDeleteNote(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want int
	}{{"success", nil, 204}, {"not found", domain.ErrNoteNotFound, 404}, {"failure", errors.New("database"), 500}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &fakeService{deleteErr: tc.err}
			h := newTestHandler(svc)
			mux := http.NewServeMux()
			mux.HandleFunc("DELETE /api/v1/notes/{id}", h.HandleDeleteNote)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/v1/notes/id", nil))
			if w.Code != tc.want {
				t.Fatalf("status = %d", w.Code)
			}
			if svc.deletedID != "id" {
				t.Fatalf("deleted id = %q", svc.deletedID)
			}
		})
	}
}

func TestHandlerCreateAndSearchNotes(t *testing.T) {
	repo := storage.NewMemoryRepository()
	svc := service.NewNoteService(repo)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := httpadapter.NewHandler(svc, logger)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/notes", handler.HandleCreateNote)
	mux.HandleFunc("GET /api/v1/notes", handler.HandleListNotes)

	body, _ := json.Marshal(map[string]string{"title": "Architecture", "content": "Searchable content"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, want %d", rec.Code, http.StatusCreated)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/notes?q=searchable", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want %d", rec.Code, http.StatusOK)
	}
	var notes []struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&notes); err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 || notes[0].Title != "Architecture" {
		t.Fatalf("unexpected list response: %#v", notes)
	}
}
