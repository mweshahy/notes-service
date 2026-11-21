package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
	// Import the pure-Go SQLite driver
	_ "modernc.org/sqlite"

	"example.com/notes/internal/domain"
)

type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository creates a new SQLite repository instance.
// Make sure to initialize your database table using the schema below.
func NewSQLiteRepository(ctx context.Context) (*SQLiteRepository, error) {
	return NewSQLiteRepositoryAt(ctx, "./data/notes.db")
}

// NewSQLiteRepositoryAt opens a SQLite repository at path and initializes its schema.
func NewSQLiteRepositoryAt(ctx context.Context, path string) (*SQLiteRepository, error) {
	if path == "" {
		return nil, fmt.Errorf("database path must not be empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := InitDB(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize database schema: %w", err)
	}
	slog.Info("SQLite repository initialized", "path", path)
	return &SQLiteRepository{db: db}, nil
}

// Close gracefully shuts down the database connection when the app stops.
func (r *SQLiteRepository) Close() error {
	return r.db.Close()
}

/*
Recommended SQLite Schema:
CREATE TABLE IF NOT EXISTS notes (

	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	content TEXT NOT NULL

);
*/
func InitDB(ctx context.Context, db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	`
	_, err := db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) Create(ctx context.Context, note *domain.Note) error {
	query := `INSERT INTO notes (id, title, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	_, err := r.db.ExecContext(ctx, query, note.ID, note.Title, note.Content, note.CreatedAt.UTC().Format(time.RFC3339Nano), note.UpdatedAt.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetByID(ctx context.Context, id string) (*domain.Note, error) {
	query := `SELECT id, title, content, created_at, updated_at FROM notes WHERE id = ?`
	row := r.db.QueryRowContext(ctx, query, id)

	var note domain.Note
	var createdAt, updatedAt string
	err := row.Scan(&note.ID, &note.Title, &note.Content, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNoteNotFound
		}
		return nil, fmt.Errorf("failed to get note by id: %w", err)
	}
	if note.CreatedAt, err = parseTime(createdAt); err != nil {
		return nil, err
	}
	if note.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return nil, err
	}

	return &note, nil
}

func (r *SQLiteRepository) Update(ctx context.Context, note *domain.Note) error {
	query := `UPDATE notes SET title = ?, content = ?, updated_at = ? WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, note.Title, note.Content, note.UpdatedAt.UTC().Format(time.RFC3339Nano), note.ID)
	if err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNoteNotFound
	}

	return nil
}

func (r *SQLiteRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM notes WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrNoteNotFound
	}

	return nil
}

func (r *SQLiteRepository) Search(ctx context.Context, query string) ([]domain.Note, error) {
	// Replicates case-insensitive substring search matching memory repository behavior
	sqlQuery := `SELECT id, title, content, created_at, updated_at FROM notes WHERE LOWER(title) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?) ORDER BY created_at, id`
	searchParam := "%" + query + "%"

	rows, err := r.db.QueryContext(ctx, sqlQuery, searchParam, searchParam)
	if err != nil {
		return nil, fmt.Errorf("failed to search notes: %w", err)
	}
	defer rows.Close()

	notes := make([]domain.Note, 0)
	for rows.Next() {
		var note domain.Note
		var createdAt, updatedAt string
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan note row: %w", err)
		}
		if note.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		if note.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return notes, nil
}

func parseTime(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse note timestamp: %w", err)
	}
	return t, nil
}
