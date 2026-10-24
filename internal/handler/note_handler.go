package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-playground/validator/v10"

	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/service"
)

// NoteHandler handles HTTP requests for notes.
//
// Node comparison:
// In Express, you write controllers `async (req, res) => { ... }`.
// In Go, handlers implement the `http.Handler` or `http.HandlerFunc` signature:
// `func(w http.ResponseWriter, r *http.Request)`.
type NoteHandler struct {
	service  service.NoteService
	validate *validator.Validate
}

// NewNoteHandler creates an instance of NoteHandler.
func NewNoteHandler(service service.NoteService, validate *validator.Validate) *NoteHandler {
	return &NoteHandler{
		service:  service,
		validate: validate,
	}
}

// RegisterRoutes registers endpoints onto the Go 1.22+ standard library ServeMux.
//
// Node comparison:
// In Go 1.22+, `http.ServeMux` natively supports HTTP methods and URL path parameters
// (e.g., "GET /api/v1/notes/{id}") without needing Express, Fastify, or third-party routers like Chi/Gorilla!
func (h *NoteHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/notes", h.Create)
	mux.HandleFunc("GET /api/v1/notes", h.List)
	mux.HandleFunc("GET /api/v1/notes/{id}", h.GetByID)
	mux.HandleFunc("PUT /api/v1/notes/{id}", h.Update)
	mux.HandleFunc("DELETE /api/v1/notes/{id}", h.Delete)
	mux.HandleFunc("GET /healthz", h.HealthCheck)
}

// Create handles POST /api/v1/notes
func (h *NoteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateNoteRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	// Validate input against struct tags
	if err := h.validate.Struct(req); err != nil {
		fieldErrors := FormatValidationErrors(err)
		WriteError(w, http.StatusUnprocessableEntity, "validation failed", fieldErrors)
		return
	}

	// Pass the request context down to service and repository
	created, err := h.service.Create(r.Context(), req)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to create note", nil)
		return
	}

	_ = WriteJSON(w, http.StatusCreated, created)
}

// GetByID handles GET /api/v1/notes/{id}
func (h *NoteHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	// Go 1.22+ PathValue extracts wildcard path parameter: {id}
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid note id: must be a positive integer", nil)
		return
	}

	note, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "note not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to retrieve note", nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, note)
}

// List handles GET /api/v1/notes?limit=20&offset=0
func (h *NoteHandler) List(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := int32(20)
	if l := query.Get("limit"); l != "" {
		if parsed, err := strconv.ParseInt(l, 10, 32); err == nil && parsed > 0 {
			limit = int32(parsed)
		}
	}

	offset := int32(0)
	if o := query.Get("offset"); o != "" {
		if parsed, err := strconv.ParseInt(o, 10, 32); err == nil && parsed >= 0 {
			offset = int32(parsed)
		}
	}

	notes, err := h.service.List(r.Context(), limit, offset)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "failed to list notes", nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]any{
		"data":   notes,
		"limit":  limit,
		"offset": offset,
	})
}

// Update handles PUT /api/v1/notes/{id}
func (h *NoteHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid note id: must be a positive integer", nil)
		return
	}

	var req model.UpdateNoteRequest
	if err := ReadJSON(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, err.Error(), nil)
		return
	}

	if err := h.validate.Struct(req); err != nil {
		fieldErrors := FormatValidationErrors(err)
		WriteError(w, http.StatusUnprocessableEntity, "validation failed", fieldErrors)
		return
	}

	updated, err := h.service.Update(r.Context(), id, req)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "note not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to update note", nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, updated)
}

// Delete handles DELETE /api/v1/notes/{id}
func (h *NoteHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid note id: must be a positive integer", nil)
		return
	}

	err = h.service.Delete(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "note not found", nil)
			return
		}
		WriteError(w, http.StatusInternalServerError, "failed to delete note", nil)
		return
	}

	// 204 No Content for successful deletion
	w.WriteHeader(http.StatusNoContent)
}

// HealthCheck handles GET /healthz
func (h *NoteHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	_ = WriteJSON(w, http.StatusOK, map[string]string{
		"status": "healthy",
	})
}
