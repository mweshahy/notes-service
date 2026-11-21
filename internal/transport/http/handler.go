package httpadapter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"example.com/notes/internal/domain"
)

type Handler struct {
	service noteService
	logger  *slog.Logger
}

type noteService interface {
	CreateNote(context.Context, string, string) (*domain.Note, error)
	GetNote(context.Context, string) (*domain.Note, error)
	List(context.Context, string) ([]domain.Note, error)
	Update(context.Context, *domain.Note) error
	Delete(context.Context, string) error
}

func NewHandler(service noteService, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

const maxRequestBodyBytes = 1 << 20

func decodeRequest(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("request body contains multiple JSON values")
		}
		return err
	}
	return nil
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}
	http.Error(w, "invalid request body", http.StatusBadRequest)
}

func (h *Handler) logError(r *http.Request, message string, err error) {
	h.logger.ErrorContext(r.Context(), message, "method", r.Method, "path", r.URL.Path, "error", err)
}

func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	if status != 0 {
		w.WriteHeader(status)
	}
	if err := json.NewEncoder(w).Encode(value); err != nil {
		h.logError(r, "failed to write JSON response", err)
	}
}

type CreateNoteRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (h *Handler) HandleCreateNote(w http.ResponseWriter, r *http.Request) {
	var req CreateNoteRequest
	if err := decodeRequest(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}

	note, err := h.service.CreateNote(r.Context(), req.Title, req.Content)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidInput) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		h.logError(r, "failed to create note", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, r, http.StatusCreated, note)
}

func (h *Handler) HandleListNotes(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	notes, err := h.service.List(r.Context(), query)
	if err != nil {
		h.logError(r, "failed to list notes", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, r, http.StatusOK, notes)
}

func (h *Handler) HandleGetNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	note, err := h.service.GetNote(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNoteNotFound) {
			http.Error(w, "Note not found", http.StatusNotFound)
			return
		}
		h.logError(r, "failed to get note", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, r, http.StatusOK, note)
}

func (h *Handler) HandleUpdateNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var input struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}

	if err := decodeRequest(w, r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}

	existing, err := h.service.GetNote(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNoteNotFound) {
			http.Error(w, "Note not found", http.StatusNotFound)
			return
		}
		h.logError(r, "failed to get note for update", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	existing.Title = input.Title
	existing.Content = input.Content
	existing.UpdatedAt = time.Now().UTC()
	if err := existing.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.Update(r.Context(), existing); err != nil {
		if errors.Is(err, domain.ErrNoteNotFound) {
			http.Error(w, "Note not found", http.StatusNotFound)
			return
		}
		h.logError(r, "failed to update note", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, r, http.StatusOK, existing)
}

func (h *Handler) HandleDeleteNote(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	err := h.service.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNoteNotFound) {
			http.Error(w, "Note not found", http.StatusNotFound)
			return
		}
		h.logError(r, "failed to delete note", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
