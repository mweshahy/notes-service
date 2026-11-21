package domain

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrNoteNotFound = errors.New("note not found")
	ErrInvalidInput = errors.New("invalid note input")
)

type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (n *Note) Validate() error {
	if n == nil || strings.TrimSpace(n.Title) == "" || strings.TrimSpace(n.Content) == "" {
		return ErrInvalidInput
	}
	return nil
}
