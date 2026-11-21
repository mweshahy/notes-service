package domain_test

import (
	"errors"
	"testing"

	"example.com/notes/internal/domain"
)

func TestNoteValidate(t *testing.T) {
	tests := []struct {
		name string
		note *domain.Note
		want error
	}{
		{name: "valid note", note: &domain.Note{Title: "Title", Content: "Content"}},
		{name: "empty title", note: &domain.Note{Content: "Content"}, want: domain.ErrInvalidInput},
		{name: "whitespace title", note: &domain.Note{Title: " \t\n", Content: "Content"}, want: domain.ErrInvalidInput},
		{name: "empty content", note: &domain.Note{Title: "Title"}, want: domain.ErrInvalidInput},
		{name: "whitespace content", note: &domain.Note{Title: "Title", Content: " \t\n"}, want: domain.ErrInvalidInput},
		{name: "nil note", want: domain.ErrInvalidInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.note.Validate()
			if !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}
