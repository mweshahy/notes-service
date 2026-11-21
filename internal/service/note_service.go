package service

import (
	"context"
	"fmt"
	"time"

	"example.com/notes/internal/domain"
	"github.com/google/uuid"
)

type NoteRepository interface {
	Create(ctx context.Context, note *domain.Note) error
	GetByID(ctx context.Context, id string) (*domain.Note, error)
	Update(ctx context.Context, note *domain.Note) error
	Delete(ctx context.Context, id string) error
	Search(ctx context.Context, query string) ([]domain.Note, error)
}

type NoteService struct {
	repo NoteRepository
}

func NewNoteService(repo NoteRepository) *NoteService {
	if repo == nil {
		panic("service: nil NoteRepository")
	}
	return &NoteService{repo: repo}
}

func (s *NoteService) CreateNote(ctx context.Context, title, content string) (*domain.Note, error) {
	now := time.Now().UTC()
	note := &domain.Note{
		ID:        uuid.New().String(),
		Title:     title,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := note.Validate(); err != nil {
		return nil, err
	}

	if err := s.repo.Create(ctx, note); err != nil {
		return nil, fmt.Errorf("failed to create note in repo: %w", err)
	}

	return note, nil
}

func (s *NoteService) GetNote(ctx context.Context, id string) (*domain.Note, error) {
	note, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get note %s: %w", id, err)
	}
	return note, nil
}

func (s *NoteService) List(ctx context.Context, query string) ([]domain.Note, error) {
	return s.repo.Search(ctx, query)
}

func (s *NoteService) Update(ctx context.Context, note *domain.Note) error {
	if err := note.Validate(); err != nil {
		return err
	}
	if err := s.repo.Update(ctx, note); err != nil {
		return fmt.Errorf("failed to update note %s: %w", note.ID, err)
	}
	return nil
}

func (s *NoteService) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete note %s: %w", id, err)
	}
	return nil
}
