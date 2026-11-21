package storage

import (
	"context"
	"strings"
	"sync"
	"time"

	"example.com/notes/internal/domain"
)

type MemoryRepository struct {
	mu    sync.RWMutex
	notes map[string]*domain.Note
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		notes: make(map[string]*domain.Note),
	}
}

func (r *MemoryRepository) Create(ctx context.Context, note *domain.Note) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notes[note.ID] = cloneNote(note)
	return nil
}

func (r *MemoryRepository) GetByID(ctx context.Context, id string) (*domain.Note, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	note, exists := r.notes[id]
	if !exists {
		return nil, domain.ErrNoteNotFound
	}
	return cloneNote(note), nil
}

func (r *MemoryRepository) Update(ctx context.Context, note *domain.Note) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.notes[note.ID]; !exists {
		return domain.ErrNoteNotFound
	}
	r.notes[note.ID] = cloneNote(note)
	return nil
}

func (r *MemoryRepository) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.notes[id]; !exists {
		return domain.ErrNoteNotFound
	}
	delete(r.notes, id)
	return nil
}

func (r *MemoryRepository) Search(ctx context.Context, query string) ([]domain.Note, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	results := make([]domain.Note, 0, len(r.notes))
	q := strings.ToLower(query)
	for _, n := range r.notes {
		if strings.Contains(strings.ToLower(n.Title), q) || strings.Contains(strings.ToLower(n.Content), q) {
			results = append(results, *cloneNote(n))
		}
	}
	return results, nil
}

func cloneNote(note *domain.Note) *domain.Note {
	if note == nil {
		return nil
	}
	copy := *note
	copy.CreatedAt = note.CreatedAt.Round(time.Nanosecond)
	copy.UpdatedAt = note.UpdatedAt.Round(time.Nanosecond)
	return &copy
}
