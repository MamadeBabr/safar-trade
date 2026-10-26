package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hossen/safar-trade/internal/middleware"
	"github.com/hossen/safar-trade/internal/model"
	"github.com/hossen/safar-trade/internal/repository"
	"github.com/hossen/safar-trade/internal/service"
)

// AuthHandler handles user registration, login, token refresh, and profile retrieval.
type AuthHandler struct {
	authService service.AuthService
	subService  service.SubscriptionService
	userRepo    repository.UserRepository
}

// NewAuthHandler constructs a new AuthHandler.
func NewAuthHandler(
	authService service.AuthService,
	subService service.SubscriptionService,
	userRepo repository.UserRepository,
) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		subService:  subService,
		userRepo:    userRepo,
	}
}

// RegisterRoutes registers OpenAPI 3.1 auth endpoints on the Huma API.
func (h *AuthHandler) RegisterRoutes(api huma.API, authMW huma.Middlewares) {
	// 1. Register User
	huma.Register(api, huma.Operation{
		OperationID: "register-user",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/register",
		Summary:     "Register new user",
		Description: "Creates a new user account with email and password, returning an access and refresh token.",
		Tags:        []string{"Authentication"},
	}, func(ctx context.Context, input *struct {
		Body model.RegisterRequest
	}) (*struct {
		Body model.AuthResponse
	}, error) {
		res, err := h.authService.Register(ctx, input.Body, "", "")
		if err != nil {
			if errors.Is(err, repository.ErrConflict) {
				return nil, huma.Error409Conflict("a user with this email already exists")
			}
			return nil, huma.Error500InternalServerError("failed to register user: " + err.Error())
		}
		return &struct{ Body model.AuthResponse }{Body: *res}, nil
	})

	// 2. Login User
	huma.Register(api, huma.Operation{
		OperationID: "login-user",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/login",
		Summary:     "Authenticate user",
		Description: "Validates credentials and returns JWT access token, refresh token, and user/subscription status.",
		Tags:        []string{"Authentication"},
	}, func(ctx context.Context, input *struct {
		Body model.LoginRequest
	}) (*struct {
		Body model.AuthResponse
	}, error) {
		res, err := h.authService.Login(ctx, input.Body, "", "")
		if err != nil {
			if errors.Is(err, service.ErrInvalidCredentials) {
				return nil, huma.Error401Unauthorized("invalid email or password")
			}
			if errors.Is(err, service.ErrUserDisabled) {
				return nil, huma.Error403Forbidden("account is disabled")
			}
			return nil, huma.Error500InternalServerError("failed to login: " + err.Error())
		}
		return &struct{ Body model.AuthResponse }{Body: *res}, nil
	})

	// 3. Refresh Access Token
	huma.Register(api, huma.Operation{
		OperationID: "refresh-token",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/refresh",
		Summary:     "Refresh access token",
		Description: "Exchanges a valid refresh token for a newly issued access token.",
		Tags:        []string{"Authentication"},
	}, func(ctx context.Context, input *struct {
		Body model.RefreshTokenRequest
	}) (*struct {
		Body model.TokenPair
	}, error) {
		pair, err := h.authService.RefreshToken(ctx, input.Body, "", "")
		if err != nil {
			return nil, huma.Error401Unauthorized("invalid or expired refresh token")
		}
		return &struct{ Body model.TokenPair }{Body: *pair}, nil
	})

	// 4. Current User Profile (/auth/me) - Protected
	huma.Register(api, huma.Operation{
		OperationID: "get-current-user",
		Method:      http.MethodGet,
		Path:        "/api/v1/auth/me",
		Summary:     "Get current user profile",
		Description: "Returns details of the currently authenticated user including active subscription.",
		Tags:        []string{"Authentication"},
		Middlewares: authMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct{}) (*struct {
		Body struct {
			User         model.UserResponse          `json:"user"`
			Subscription *model.SubscriptionResponse `json:"subscription,omitempty"`
		}
	}, error) {
		claims := middleware.GetAuthUser(ctx)
		if claims == nil {
			return nil, huma.Error401Unauthorized("unauthorized")
		}

		user, err := h.userRepo.GetByID(ctx, claims.UserID)
		if err != nil {
			return nil, huma.Error404NotFound("user profile not found")
		}

		var subResp *model.SubscriptionResponse
		if h.subService != nil {
			subResp, _ = h.subService.GetActiveSubscription(ctx, user.ID)
		}

		return &struct {
			Body struct {
				User         model.UserResponse          `json:"user"`
				Subscription *model.SubscriptionResponse `json:"subscription,omitempty"`
			}
		}{
			Body: struct {
				User         model.UserResponse          `json:"user"`
				Subscription *model.SubscriptionResponse `json:"subscription,omitempty"`
			}{
				User:         user.ToResponse(),
				Subscription: subResp,
			},
		}, nil
	})

	// 5. Logout - Protected
	huma.Register(api, huma.Operation{
		OperationID: "logout-user",
		Method:      http.MethodPost,
		Path:        "/api/v1/auth/logout",
		Summary:     "Logout and revoke session",
		Description: "Revokes the active refresh token session and clears associated caches.",
		Tags:        []string{"Authentication"},
		Middlewares: authMW,
		Security: []map[string][]string{
			{"bearerAuth": {}},
		},
	}, func(ctx context.Context, input *struct {
		Body struct {
			RefreshToken string `json:"refresh_token" doc:"Optional refresh token to revoke specifically"`
		}
	}) (*struct {
		Body struct {
			Message string `json:"message" example:"logged out successfully"`
		}
	}, error) {
		userID := middleware.GetUserID(ctx)
		_ = h.authService.Logout(ctx, input.Body.RefreshToken, userID)
		return &struct {
			Body struct {
				Message string `json:"message" example:"logged out successfully"`
			}
		}{
			Body: struct {
				Message string `json:"message" example:"logged out successfully"`
			}{
				Message: "logged out successfully",
			},
		}, nil
	})
}
