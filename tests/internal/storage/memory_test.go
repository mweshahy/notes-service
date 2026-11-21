package storage_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/notes/internal/domain"
	"example.com/notes/internal/storage"
)

func TestMemoryRepositoryCRUD(t *testing.T) {
	ctx := context.Background()
	note := &domain.Note{ID: "one", Title: "Title", Content: "Body"}
	tests := []struct {
		name string
		run  func(*testing.T, *storage.MemoryRepository)
	}{
		{name: "create and get return copies", run: func(t *testing.T, repo *storage.MemoryRepository) {
			if err := repo.Create(ctx, note); err != nil {
				t.Fatal(err)
			}
			note.Title = "mutated"
			got, err := repo.GetByID(ctx, "one")
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != "Title" {
				t.Fatalf("Title = %q, want original", got.Title)
			}
			got.Title = "also mutated"
			again, err := repo.GetByID(ctx, "one")
			if err != nil {
				t.Fatal(err)
			}
			if again.Title != "Title" {
				t.Fatalf("stored title changed: %q", again.Title)
			}
		}},
		{name: "update existing", run: func(t *testing.T, repo *storage.MemoryRepository) {
			if err := repo.Create(ctx, note); err != nil {
				t.Fatal(err)
			}
			updated := &domain.Note{ID: "one", Title: "Updated", Content: "Body"}
			if err := repo.Update(ctx, updated); err != nil {
				t.Fatal(err)
			}
			got, err := repo.GetByID(ctx, "one")
			if err != nil {
				t.Fatal(err)
			}
			if got.Title != updated.Title {
				t.Fatalf("Title = %q, want %q", got.Title, updated.Title)
			}
		}},
		{name: "missing get update and delete", run: func(t *testing.T, repo *storage.MemoryRepository) {
			if _, err := repo.GetByID(ctx, "missing"); !errors.Is(err, domain.ErrNoteNotFound) {
				t.Fatalf("GetByID error = %v", err)
			}
			if err := repo.Update(ctx, &domain.Note{ID: "missing"}); !errors.Is(err, domain.ErrNoteNotFound) {
				t.Fatalf("Update error = %v", err)
			}
			if err := repo.Delete(ctx, "missing"); !errors.Is(err, domain.ErrNoteNotFound) {
				t.Fatalf("Delete error = %v", err)
			}
		}},
		{name: "delete existing", run: func(t *testing.T, repo *storage.MemoryRepository) {
			if err := repo.Create(ctx, note); err != nil {
				t.Fatal(err)
			}
			if err := repo.Delete(ctx, "one"); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.GetByID(ctx, "one"); !errors.Is(err, domain.ErrNoteNotFound) {
				t.Fatalf("GetByID error = %v", err)
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { tt.run(t, storage.NewMemoryRepository()) })
	}
}

func TestMemoryRepositoryContextAndSearch(t *testing.T) {
	repo := storage.NewMemoryRepository()
	ctx := context.Background()
	for _, note := range []*domain.Note{
		{ID: "a", Title: "Go Notes", Content: "Concurrency"},
		{ID: "b", Title: "Shopping", Content: "Bread"},
	} {
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{name: "case insensitive title", query: "gO nOtEs", want: 1},
		{name: "content match", query: "BREAD", want: 1},
		{name: "empty query matches all", query: "", want: 2},
		{name: "no match", query: "absent", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Search(ctx, tt.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tt.want {
				t.Fatalf("Search() got %d results, want %d", len(got), tt.want)
			}
		})
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for name, operation := range map[string]func() error{
		"create": func() error { return repo.Create(canceled, &domain.Note{ID: "c"}) },
		"get":    func() error { _, err := repo.GetByID(canceled, "a"); return err },
		"update": func() error { return repo.Update(canceled, &domain.Note{ID: "a"}) },
		"delete": func() error { return repo.Delete(canceled, "a") },
		"search": func() error { _, err := repo.Search(canceled, ""); return err },
	} {
		t.Run("canceled context/"+name, func(t *testing.T) {
			if err := operation(); !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestSQLiteRepositoryCRUDAndSearch(t *testing.T) {
	t.Run("empty path is rejected", func(t *testing.T) {
		if _, err := storage.NewSQLiteRepositoryAt(context.Background(), ""); err == nil {
			t.Fatal("expected empty database path error")
		}
	})
	t.Run("operations", func(t *testing.T) {
		path := t.TempDir() + "/nested/notes.db"
		repo, err := storage.NewSQLiteRepositoryAt(context.Background(), path)
		if err != nil {
			t.Fatal(err)
		}
		defer repo.Close()
		ctx := context.Background()
		now := time.Now().UTC().Truncate(time.Microsecond)
		note := &domain.Note{ID: "one", Title: "Go Notes", Content: "Concurrency", CreatedAt: now, UpdatedAt: now}
		if err := repo.Create(ctx, note); err != nil {
			t.Fatal(err)
		}
		got, err := repo.GetByID(ctx, "one")
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != note.Title || !got.CreatedAt.Equal(now) {
			t.Fatalf("GetByID() = %#v", got)
		}
		updated := *note
		updated.Title = "Updated"
		updated.UpdatedAt = now.Add(time.Minute)
		if err := repo.Update(ctx, &updated); err != nil {
			t.Fatal(err)
		}
		results, err := repo.Search(ctx, "UPDATED")
		if err != nil || len(results) != 1 || results[0].Title != "Updated" {
			t.Fatalf("Search() = %#v, %v", results, err)
		}
		if err := repo.Update(ctx, &domain.Note{ID: "missing"}); !errors.Is(err, domain.ErrNoteNotFound) {
			t.Fatalf("missing update error = %v", err)
		}
		if err := repo.Delete(ctx, "missing"); !errors.Is(err, domain.ErrNoteNotFound) {
			t.Fatalf("missing delete error = %v", err)
		}
		if err := repo.Delete(ctx, "one"); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.GetByID(ctx, "one"); !errors.Is(err, domain.ErrNoteNotFound) {
			t.Fatalf("deleted get error = %v", err)
		}
	})
}
