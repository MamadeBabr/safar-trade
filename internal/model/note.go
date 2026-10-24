// Package model defines core domain entities and request/response DTOs.
//
// Node comparison:
// In TypeScript, you would define `interface Note { id: number; ... }` and a Zod schema
// `z.object({ title: z.string().min(1) })`.
// In Go, struct fields use struct tags:
// - `json:"..."` specifies serialization keys (similar to class-transformer / JSON.stringify)
// - `validate:"..."` provides declarative constraints enforced by go-playground/validator
package model

import (
	"time"
)

// Note represents the core domain model for a note in the system.
type Note struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateNoteRequest defines the payload for creating a new note.
// Struct tags instruct validator how to validate each field.
type CreateNoteRequest struct {
	UserID   *int64 `json:"user_id,omitempty" validate:"omitempty,gt=0"`
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Content  string `json:"content" validate:"required"`
	Category string `json:"category,omitempty" validate:"omitempty,max=50"`
}

// UpdateNoteRequest defines the payload for updating an existing note.
type UpdateNoteRequest struct {
	Title    string `json:"title" validate:"required,min=1,max=255"`
	Content  string `json:"content" validate:"required"`
	Category string `json:"category,omitempty" validate:"omitempty,max=50"`
}

// NoteResponse is the client-facing representation of a note.
type NoteResponse struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ToResponse converts a domain Note into an API NoteResponse.
func (n *Note) ToResponse() NoteResponse {
	return NoteResponse{
		ID:        n.ID,
		UserID:    n.UserID,
		Title:     n.Title,
		Content:   n.Content,
		Category:  n.Category,
		CreatedAt: n.CreatedAt,
		UpdatedAt: n.UpdatedAt,
	}
}
