package service_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/service"
)

// mockNoteRepository is an in-memory mock implementation of repository.NoteRepository.
//
// Node comparison:
// In Jest/Vitest, you write `jest.fn()` or `mockResolvedValue()`.
// In Go without extra mock libraries, defining a struct with function fields is idiomatic:
// each test case can configure precisely what the mock returns.
type mockNoteRepository struct {
	createFn  func(ctx context.Context, note *model.Note) (*model.Note, error)
	getByIDFn func(ctx context.Context, id int64) (*model.Note, error)
	listFn    func(ctx context.Context, limit, offset int32) ([]model.Note, error)
	updateFn  func(ctx context.Context, note *model.Note) (*model.Note, error)
	deleteFn  func(ctx context.Context, id int64) error
}

func (m *mockNoteRepository) Create(ctx context.Context, note *model.Note) (*model.Note, error) {
	if m.createFn != nil {
		return m.createFn(ctx, note)
	}
	return nil, errors.New("createFn not implemented")
}

func (m *mockNoteRepository) GetByID(ctx context.Context, id int64) (*model.Note, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("getByIDFn not implemented")
}

func (m *mockNoteRepository) List(ctx context.Context, limit, offset int32) ([]model.Note, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, errors.New("listFn not implemented")
}

func (m *mockNoteRepository) Update(ctx context.Context, note *model.Note) (*model.Note, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, note)
	}
	return nil, errors.New("updateFn not implemented")
}

func (m *mockNoteRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("deleteFn not implemented")
}

// Compile-time check: verify mock satisfies repository interface
var _ repository.NoteRepository = (*mockNoteRepository)(nil)

// TestNoteService_Create demonstrates a classic Go table-driven unit test.
//
// Node comparison:
// In Jest, you use `test.each([ ... ])`.
// In Go, table-driven tests use a slice of anonymous structs with subtests `t.Run(tc.name, ...)`.
func TestNoteService_Create(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		req         model.CreateNoteRequest
		mockSetup   func(m *mockNoteRepository)
		expected    *model.NoteResponse
		expectedErr error
	}{
		{
			name: "successfully creates note with default category",
			req: model.CreateNoteRequest{
				Title:   "Go Table Tests",
				Content: "Testing in Go is built into the standard library.",
				// Category is intentionally empty to test default assignment
			},
			mockSetup: func(m *mockNoteRepository) {
				m.createFn = func(ctx context.Context, note *model.Note) (*model.Note, error) {
					// Verify default category logic in service
					if note.Category != "general" {
						t.Errorf("expected category 'general', got %q", note.Category)
					}
					return &model.Note{
						ID:        1,
						Title:     note.Title,
						Content:   note.Content,
						Category:  note.Category,
						CreatedAt: now,
						UpdatedAt: now,
					}, nil
				}
			},
			expected: &model.NoteResponse{
				ID:        1,
				Title:     "Go Table Tests",
				Content:   "Testing in Go is built into the standard library.",
				Category:  "general",
				CreatedAt: now,
				UpdatedAt: now,
			},
			expectedErr: nil,
		},
		{
			name: "successfully creates note with custom category",
			req: model.CreateNoteRequest{
				Title:    "Custom Category Note",
				Content:  "Note content",
				Category: "backend",
			},
			mockSetup: func(m *mockNoteRepository) {
				m.createFn = func(ctx context.Context, note *model.Note) (*model.Note, error) {
					return &model.Note{
						ID:        2,
						Title:     note.Title,
						Content:   note.Content,
						Category:  note.Category,
						CreatedAt: now,
						UpdatedAt: now,
					}, nil
				}
			},
			expected: &model.NoteResponse{
				ID:        2,
				Title:     "Custom Category Note",
				Content:   "Note content",
				Category:  "backend",
				CreatedAt: now,
				UpdatedAt: now,
			},
			expectedErr: nil,
		},
		{
			name: "propagates repository database error",
			req: model.CreateNoteRequest{
				Title:   "Failing Note",
				Content: "Should trigger error",
			},
			mockSetup: func(m *mockNoteRepository) {
				m.createFn = func(ctx context.Context, note *model.Note) (*model.Note, error) {
					return nil, errors.New("db connection failure")
				}
			},
			expected:    nil,
			expectedErr: errors.New("db connection failure"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockRepo := &mockNoteRepository{}
			tc.mockSetup(mockRepo)

			svc := service.NewNoteService(mockRepo)
			res, err := svc.Create(context.Background(), tc.req)

			if tc.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", tc.expectedErr)
				}
				if err.Error() != tc.expectedErr.Error() {
					t.Fatalf("expected error message %q, got %q", tc.expectedErr.Error(), err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(res, tc.expected) {
				t.Errorf("expected %+v, got %+v", tc.expected, res)
			}
		})
	}
}

func TestNoteService_GetByID(t *testing.T) {
	now := time.Now()

	t.Run("returns note when found", func(t *testing.T) {
		mockRepo := &mockNoteRepository{
			getByIDFn: func(ctx context.Context, id int64) (*model.Note, error) {
				return &model.Note{
					ID:        id,
					Title:     "Found note",
					Content:   "Content",
					Category:  "general",
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
		}

		svc := service.NewNoteService(mockRepo)
		res, err := svc.GetByID(context.Background(), 42)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != 42 || res.Title != "Found note" {
			t.Errorf("unexpected response: %+v", res)
		}
	})

	t.Run("returns ErrNotFound when note does not exist", func(t *testing.T) {
		mockRepo := &mockNoteRepository{
			getByIDFn: func(ctx context.Context, id int64) (*model.Note, error) {
				return nil, repository.ErrNotFound
			},
		}

		svc := service.NewNoteService(mockRepo)
		_, err := svc.GetByID(context.Background(), 999)

		if !errors.Is(err, repository.ErrNotFound) {
			t.Fatalf("expected ErrNotFound, got %v", err)
		}
	})
}
