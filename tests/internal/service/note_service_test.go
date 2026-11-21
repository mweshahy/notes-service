package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/notes/internal/domain"
	"example.com/notes/internal/service"
	"example.com/notes/internal/storage"
)

type stubRepository struct {
	createErr, getErr, updateErr, deleteErr, searchErr error
	note                                               *domain.Note
	searchResult                                       []domain.Note
	created                                            *domain.Note
	updated                                            *domain.Note
	deletedID, searchQuery                             string
}

func (s *stubRepository) Create(_ context.Context, n *domain.Note) error {
	s.created = n
	return s.createErr
}
func (s *stubRepository) GetByID(_ context.Context, _ string) (*domain.Note, error) {
	return s.note, s.getErr
}
func (s *stubRepository) Update(_ context.Context, n *domain.Note) error {
	s.updated = n
	return s.updateErr
}
func (s *stubRepository) Delete(_ context.Context, id string) error {
	s.deletedID = id
	return s.deleteErr
}
func (s *stubRepository) Search(_ context.Context, query string) ([]domain.Note, error) {
	s.searchQuery = query
	return s.searchResult, s.searchErr
}

func TestNewNoteServiceNilRepository(t *testing.T) {
	t.Run("panics for nil repository", func(t *testing.T) {
		defer func() {
			if recover() == nil {
				t.Fatal("NewNoteService(nil) did not panic")
			}
		}()
		service.NewNoteService(nil)
	})
}

func TestNoteServiceCreateNote(t *testing.T) {
	repoErr := errors.New("storage failure")
	tests := []struct {
		name, title, content string
		repoErr              error
		wantErr              error
	}{
		{name: "valid note", title: "Title", content: "Body"},
		{name: "empty title", content: "body", wantErr: domain.ErrInvalidInput},
		{name: "empty content", title: "title", wantErr: domain.ErrInvalidInput},
		{name: "repository failure", title: "title", content: "body", repoErr: repoErr, wantErr: repoErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubRepository{createErr: tt.repoErr}
			got, err := service.NewNoteService(repo).CreateNote(context.Background(), tt.title, tt.content)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CreateNote() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if got != nil {
					t.Fatalf("note = %#v, want nil", got)
				}
				if errors.Is(tt.wantErr, domain.ErrInvalidInput) && repo.created != nil {
					t.Fatal("repository called for invalid input")
				}
				return
			}
			if got == nil || got.ID == "" || got.Title != tt.title || got.Content != tt.content {
				t.Fatalf("unexpected created note: %#v", got)
			}
			if _, err := time.Parse(time.RFC3339Nano, got.CreatedAt.Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			if !got.CreatedAt.Equal(got.UpdatedAt) || got.CreatedAt.Location() != time.UTC {
				t.Fatalf("timestamps not equal UTC: %#v", got)
			}
		})
	}
}

func TestNoteServiceCreateAndListSearchIntegration(t *testing.T) {
	svc := service.NewNoteService(storage.NewMemoryRepository())
	ctx := context.Background()
	for _, note := range []struct{ title, content string }{
		{"Go architecture", "layered notes service"},
		{"Shopping", "milk and bread"},
	} {
		if _, err := svc.CreateNote(ctx, note.title, note.content); err != nil {
			t.Fatalf("CreateNote(%q): %v", note.title, err)
		}
	}

	got, err := svc.List(ctx, "LAYERED")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Title != "Go architecture" {
		t.Fatalf("List search returned %#v", got)
	}
}

func TestNoteServiceRepositoryOperations(t *testing.T) {
	repoErr := errors.New("repository failure")
	note := &domain.Note{ID: "id", Title: "Title", Content: "Body"}
	t.Run("get success and error", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			err  error
			want error
		}{{"success", nil, nil}, {"not found", domain.ErrNoteNotFound, domain.ErrNoteNotFound}, {"failure", repoErr, repoErr}} {
			t.Run(tc.name, func(t *testing.T) {
				got, err := service.NewNoteService(&stubRepository{note: note, getErr: tc.err}).GetNote(context.Background(), "id")
				if !errors.Is(err, tc.want) {
					t.Fatalf("error = %v, want %v", err, tc.want)
				}
				if tc.want == nil && got != note {
					t.Fatalf("note = %p, want %p", got, note)
				}
			})
		}
	})
	t.Run("list passes query and repository result", func(t *testing.T) {
		want := []domain.Note{*note}
		repo := &stubRepository{searchResult: want}
		got, err := service.NewNoteService(repo).List(context.Background(), "needle")
		if err != nil || repo.searchQuery != "needle" || len(got) != 1 {
			t.Fatalf("List() = %#v, %v; query %q", got, err, repo.searchQuery)
		}
	})
	t.Run("update validates before repository", func(t *testing.T) {
		repo := &stubRepository{}
		if err := service.NewNoteService(repo).Update(context.Background(), nil); !errors.Is(err, domain.ErrInvalidInput) {
			t.Fatalf("Update(nil) error = %v", err)
		}
		if repo.updated != nil {
			t.Fatal("repository called for invalid note")
		}
	})
	for _, tc := range []struct {
		name    string
		repoErr error
		want    error
	}{{"update success", nil, nil}, {"update not found", domain.ErrNoteNotFound, domain.ErrNoteNotFound}, {"update failure", repoErr, repoErr}} {
		t.Run(tc.name, func(t *testing.T) {
			err := service.NewNoteService(&stubRepository{updateErr: tc.repoErr}).Update(context.Background(), note)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name    string
		repoErr error
		want    error
	}{{"delete success", nil, nil}, {"delete not found", domain.ErrNoteNotFound, domain.ErrNoteNotFound}, {"delete failure", repoErr, repoErr}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubRepository{deleteErr: tc.repoErr}
			err := service.NewNoteService(repo).Delete(context.Background(), "id")
			if !errors.Is(err, tc.want) || repo.deletedID != "id" {
				t.Fatalf("Delete() error = %v, id = %q", err, repo.deletedID)
			}
		})
	}
}
