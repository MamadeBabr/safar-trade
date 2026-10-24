// Package service implements the core business logic layer.
//
// Node comparison:
// In NestJS or Express service classes, you might inject a repository directly.
// In Go, services depend on *interfaces* (NoteRepository) rather than concrete database structs.
// This allows trivial mocking in unit tests without spinning up a test database or using jest.mock().
package service

import (
	"context"
	"strings"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
)

// NoteService defines the business operations available for notes.
type NoteService interface {
	Create(ctx context.Context, req model.CreateNoteRequest) (*model.NoteResponse, error)
	GetByID(ctx context.Context, id int64) (*model.NoteResponse, error)
	List(ctx context.Context, limit, offset int32) ([]model.NoteResponse, error)
	Update(ctx context.Context, id int64, req model.UpdateNoteRequest) (*model.NoteResponse, error)
	Delete(ctx context.Context, id int64) error
}

type noteService struct {
	repo repository.NoteRepository
}

// Compile-time check: ensures *noteService implements NoteService.
var _ NoteService = (*noteService)(nil)

// NewNoteService creates a new NoteService instance with repository dependency injection.
func NewNoteService(repo repository.NoteRepository) NoteService {
	return &noteService{repo: repo}
}

func (s *noteService) Create(ctx context.Context, req model.CreateNoteRequest) (*model.NoteResponse, error) {
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "general"
	}

	note := &model.Note{
		UserID:   req.UserID,
		Title:    strings.TrimSpace(req.Title),
		Content:  strings.TrimSpace(req.Content),
		Category: category,
	}

	created, err := s.repo.Create(ctx, note)
	if err != nil {
		return nil, err
	}

	resp := created.ToResponse()
	return &resp, nil
}

func (s *noteService) GetByID(ctx context.Context, id int64) (*model.NoteResponse, error) {
	note, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp := note.ToResponse()
	return &resp, nil
}

func (s *noteService) List(ctx context.Context, limit, offset int32) ([]model.NoteResponse, error) {
	// Enforce pagination boundaries
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	notes, err := s.repo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	responses := make([]model.NoteResponse, len(notes))
	for i := range notes {
		responses[i] = notes[i].ToResponse()
	}
	return responses, nil
}

func (s *noteService) Update(ctx context.Context, id int64, req model.UpdateNoteRequest) (*model.NoteResponse, error) {
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = "general"
	}

	note := &model.Note{
		ID:       id,
		Title:    strings.TrimSpace(req.Title),
		Content:  strings.TrimSpace(req.Content),
		Category: category,
	}

	updated, err := s.repo.Update(ctx, note)
	if err != nil {
		return nil, err
	}

	resp := updated.ToResponse()
	return &resp, nil
}

func (s *noteService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
