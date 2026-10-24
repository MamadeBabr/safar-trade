package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/hossen/safar-trade/internal/ingestion/forexfactory"
	"github.com/hossen/safar-trade/internal/repository"
)

// IngestionHandler handles on-demand ingestion requests and economic event queries.
type IngestionHandler struct {
	ingester forexfactory.Ingester
	repo     repository.EconomicEventRepository
	jsonPath string
}

// NewIngestionHandler creates a new IngestionHandler instance.
func NewIngestionHandler(ingester forexfactory.Ingester, repo repository.EconomicEventRepository, jsonPath string) *IngestionHandler {
	return &IngestionHandler{
		ingester: ingester,
		repo:     repo,
		jsonPath: jsonPath,
	}
}

// RegisterRoutes registers the ingestion and economic event endpoints.
func (h *IngestionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/ingestion/forex-factory", h.TriggerForexFactoryIngestion)
	mux.HandleFunc("GET /api/v1/economic-events", h.ListEconomicEvents)
	mux.HandleFunc("GET /api/v1/economic-events/{id}", h.GetEconomicEventByID)
}

// TriggerForexFactoryIngestion handles POST /api/v1/ingestion/forex-factory
func (h *IngestionHandler) TriggerForexFactoryIngestion(w http.ResponseWriter, r *http.Request) {
	res, err := h.ingester.Ingest(r.Context())
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "forex factory ingestion failed: "+err.Error(), nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, res)
}

// ListEconomicEvents handles GET /api/v1/economic-events?limit=20&offset=0
func (h *IngestionHandler) ListEconomicEvents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	limit := int32(50)
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

	events, err := h.repo.List(r.Context(), limit, offset)
	if err != nil {
		// Resilience: Fall back to reading from local JSON cache if database is offline!
		if h.jsonPath != "" {
			jsonEvents, jsonErr := forexfactory.ReadEventsFromJSON(h.jsonPath, limit, offset)
			if jsonErr == nil && len(jsonEvents) > 0 {
				_ = WriteJSON(w, http.StatusOK, map[string]any{
					"data":   jsonEvents,
					"limit":  limit,
					"offset": offset,
					"count":  len(jsonEvents),
					"source": "json_fallback (database offline)",
				})
				return
			}
		}

		WriteError(w, http.StatusInternalServerError, "failed to list economic events: "+err.Error(), nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, map[string]any{
		"data":   events,
		"limit":  limit,
		"offset": offset,
		"count":  len(events),
		"source": "database",
	})
}

// GetEconomicEventByID handles GET /api/v1/economic-events/{id}
func (h *IngestionHandler) GetEconomicEventByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		WriteError(w, http.StatusBadRequest, "invalid event id: must be a positive integer", nil)
		return
	}

	event, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "economic event not found", nil)
			return
		}

		// Resilience: If database query failed (e.g. connection refused), check JSON fallback
		if h.jsonPath != "" {
			jsonEvents, jsonErr := forexfactory.ReadEventsFromJSON(h.jsonPath, 1000, 0)
			if jsonErr == nil {
				for _, e := range jsonEvents {
					if e.ID == id {
						_ = WriteJSON(w, http.StatusOK, e)
						return
					}
				}
			}
		}

		WriteError(w, http.StatusInternalServerError, "failed to retrieve economic event", nil)
		return
	}

	_ = WriteJSON(w, http.StatusOK, event)
}
