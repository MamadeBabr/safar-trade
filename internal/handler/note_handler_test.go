package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"

	"github.com/hossen/safar-trade/internal/handler"
	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/service"
)

// mockNoteService mocks service.NoteService for handler unit testing.
//
// Node comparison:
// In Node.js / Express, you might use `sinon.stub()` or `jest.spyOn(service, 'create')`.
// In Go, writing a mock struct satisfying the interface allows clean in-memory tests
// using standard net/http/httptest without spinning up an actual network socket.
type mockNoteService struct {
	createFn  func(ctx context.Context, req model.CreateNoteRequest) (*model.NoteResponse, error)
	getByIDFn func(ctx context.Context, id int64) (*model.NoteResponse, error)
	listFn    func(ctx context.Context, limit, offset int32) ([]model.NoteResponse, error)
	updateFn  func(ctx context.Context, id int64, req model.UpdateNoteRequest) (*model.NoteResponse, error)
	deleteFn  func(ctx context.Context, id int64) error
}

func (m *mockNoteService) Create(ctx context.Context, req model.CreateNoteRequest) (*model.NoteResponse, error) {
	if m.createFn != nil {
		return m.createFn(ctx, req)
	}
	return nil, errors.New("createFn not implemented")
}

func (m *mockNoteService) GetByID(ctx context.Context, id int64) (*model.NoteResponse, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("getByIDFn not implemented")
}

func (m *mockNoteService) List(ctx context.Context, limit, offset int32) ([]model.NoteResponse, error) {
	if m.listFn != nil {
		return m.listFn(ctx, limit, offset)
	}
	return nil, errors.New("listFn not implemented")
}

func (m *mockNoteService) Update(ctx context.Context, id int64, req model.UpdateNoteRequest) (*model.NoteResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, req)
	}
	return nil, errors.New("updateFn not implemented")
}

func (m *mockNoteService) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return errors.New("deleteFn not implemented")
}

// Compile-time interface check
var _ service.NoteService = (*mockNoteService)(nil)

func setupTestRouter(mock *mockNoteService) *http.ServeMux {
	validate := validator.New()
	h := handler.NewNoteHandler(mock, validate)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return mux
}

func TestNoteHandler_Create(t *testing.T) {
	now := time.Now()

	t.Run("valid payload returns 201 Created and JSON", func(t *testing.T) {
		mockSvc := &mockNoteService{
			createFn: func(ctx context.Context, req model.CreateNoteRequest) (*model.NoteResponse, error) {
				return &model.NoteResponse{
					ID:        10,
					Title:     req.Title,
					Content:   req.Content,
					Category:  "general",
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
		}

		router := setupTestRouter(mockSvc)

		body := []byte(`{"title":"Learning Go","content":"Idiomatic Go for Node devs"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Execute the request through the router
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, rec.Code, rec.Body.String())
		}

		var resp model.NoteResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response JSON: %v", err)
		}

		if resp.ID != 10 || resp.Title != "Learning Go" {
			t.Errorf("unexpected response content: %+v", resp)
		}
	})

	t.Run("missing required fields returns 422 Unprocessable Entity", func(t *testing.T) {
		mockSvc := &mockNoteService{}
		router := setupTestRouter(mockSvc)

		// Title is missing (violates validate:"required")
		body := []byte(`{"content":"No title here"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, rec.Code)
		}

		var errResp handler.StandardErrorResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to parse error response: %v", err)
		}

		if errResp.Error.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected error code %d, got %d", http.StatusUnprocessableEntity, errResp.Error.Code)
		}
	})

	t.Run("malformed JSON body returns 400 Bad Request", func(t *testing.T) {
		mockSvc := &mockNoteService{}
		router := setupTestRouter(mockSvc)

		body := []byte(`{invalid-json}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/notes", bytes.NewReader(body))
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestNoteHandler_GetByID(t *testing.T) {
	now := time.Now()

	t.Run("valid id returns 200 OK and Note", func(t *testing.T) {
		mockSvc := &mockNoteService{
			getByIDFn: func(ctx context.Context, id int64) (*model.NoteResponse, error) {
				return &model.NoteResponse{
					ID:        id,
					Title:     "Fetched note",
					Content:   "Content",
					Category:  "general",
					CreatedAt: now,
					UpdatedAt: now,
				}, nil
			},
		}

		router := setupTestRouter(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notes/123", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp model.NoteResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if resp.ID != 123 || resp.Title != "Fetched note" {
			t.Errorf("unexpected note content: %+v", resp)
		}
	})

	t.Run("non-existent note returns 404 Not Found", func(t *testing.T) {
		mockSvc := &mockNoteService{
			getByIDFn: func(ctx context.Context, id int64) (*model.NoteResponse, error) {
				return nil, repository.ErrNotFound
			},
		}

		router := setupTestRouter(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notes/9999", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("non-numeric id returns 400 Bad Request", func(t *testing.T) {
		mockSvc := &mockNoteService{}
		router := setupTestRouter(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notes/not-a-number", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestNoteHandler_HealthCheck(t *testing.T) {
	mockSvc := &mockNoteService{}
	router := setupTestRouter(mockSvc)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var body map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %q", body["status"])
	}
}
