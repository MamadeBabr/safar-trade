package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hossen/safar-trade/internal/middleware"
	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/service"
)

// SubscriptionHandler handles listing packages and purchasing/cancelling subscriptions.
type SubscriptionHandler struct {
	subService service.SubscriptionService
}

// NewSubscriptionHandler creates a new SubscriptionHandler.
func NewSubscriptionHandler(subService service.SubscriptionService) *SubscriptionHandler {
	return &SubscriptionHandler{subService: subService}
}

// RegisterRoutes registers OpenAPI 3.1 package and subscription endpoints.
func (h *SubscriptionHandler) RegisterRoutes(api huma.API, authMW huma.Middlewares) {
	// 1. List Available Packages (Public)
	huma.Register(api, huma.Operation{
		OperationID: "list-packages",
		Method:      http.MethodGet,
		Path:        "/api/v1/packages",
		Summary:     "List subscription packages",
		Description: "Returns all active subscription packages available for purchase.",
		Tags:        []string{"Subscriptions"},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body []model.PackageResponse
	}, error) {
		packages, err := h.subService.ListPackages(ctx)
		if err != nil {
			return nil, huma.Error500InternalServerError("failed to retrieve packages: " + err.Error())
		}
		return &struct{ Body []model.PackageResponse }{Body: packages}, nil
	})

	// 2. Subscribe to Package (Auth required)
	huma.Register(api, huma.Operation{
		OperationID: "subscribe-package",
		Method:      http.MethodPost,
		Path:        "/api/v1/subscriptions",
		Summary:     "Subscribe to a package",
		Description: "Activates a subscription for the authenticated user and invalidates any cached status.",
		Tags:        []string{"Subscriptions"},
		Middlewares: authMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct {
		Body model.SubscribeRequest
	}) (*struct {
		Body model.SubscriptionResponse
	}, error) {
		userID := middleware.GetUserID(ctx)
		if userID == 0 {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		sub, err := h.subService.Subscribe(ctx, userID, input.Body)
		if err != nil {
			if errors.Is(err, service.ErrPackageNotFound) {
				return nil, huma.Error404NotFound("package not found")
			}
			return nil, huma.Error500InternalServerError("failed to create subscription: " + err.Error())
		}

		return &struct{ Body model.SubscriptionResponse }{Body: *sub}, nil
	})

	// 3. Get Current Subscription (Auth required, cached in Redis)
	huma.Register(api, huma.Operation{
		OperationID: "get-current-subscription",
		Method:      http.MethodGet,
		Path:        "/api/v1/subscriptions/current",
		Summary:     "Get current subscription",
		Description: "Retrieves the user's active subscription status (served from Redis cache with PostgreSQL fallback).",
		Tags:        []string{"Subscriptions"},
		Middlewares: authMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body model.SubscriptionResponse
	}, error) {
		userID := middleware.GetUserID(ctx)
		if userID == 0 {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		sub, err := h.subService.GetActiveSubscription(ctx, userID)
		if err != nil {
			if errors.Is(err, service.ErrSubscriptionNotFound) {
				return nil, huma.Error404NotFound("no active subscription found")
			}
			return nil, huma.Error500InternalServerError("failed to check subscription: " + err.Error())
		}

		return &struct{ Body model.SubscriptionResponse }{Body: *sub}, nil
	})

	// 4. Cancel Active Subscription (Auth required)
	huma.Register(api, huma.Operation{
		OperationID: "cancel-subscription",
		Method:      http.MethodPost,
		Path:        "/api/v1/subscriptions/cancel",
		Summary:     "Cancel active subscription",
		Description: "Cancels the active subscription and immediately purges Redis cache.",
		Tags:        []string{"Subscriptions"},
		Middlewares: authMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			Message string `json:"message" example:"subscription successfully canceled"`
		}
	}, error) {
		userID := middleware.GetUserID(ctx)
		if userID == 0 {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		if err := h.subService.Cancel(ctx, userID); err != nil {
			if errors.Is(err, service.ErrSubscriptionNotFound) {
				return nil, huma.Error404NotFound("no active subscription to cancel")
			}
			return nil, huma.Error500InternalServerError("failed to cancel subscription: " + err.Error())
		}

		return &struct {
			Body struct {
				Message string `json:"message" example:"subscription successfully canceled"`
			}
		}{
			Body: struct {
				Message string `json:"message" example:"subscription successfully canceled"`
			}{
				Message: "subscription successfully canceled",
			},
		}, nil
	})
}
