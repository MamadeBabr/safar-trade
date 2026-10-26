package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/hossen/safar-trade/internal/service"
)

const (
	authUserKey contextKey = "auth_user"
)

// Authenticate verifies the incoming Bearer JWT access token and injects claims into context.
// Used for standard net/http handlers.
func Authenticate(authService service.AuthService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSONError(w, http.StatusUnauthorized, "invalid authorization format, expected 'Bearer <token>'")
				return
			}

			tokenString := parts[1]
			claims, err := authService.ValidateAccessToken(tokenString)
			if err != nil {
				writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			// Store claims in request context
			ctx := context.WithValue(r.Context(), authUserKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireSubscription ensures that the authenticated user has an active, non-expired subscription.
// Subscription status is checked against the Redis cache with automatic PostgreSQL fallback.
func RequireSubscription(subService service.SubscriptionService) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetAuthUser(r.Context())
			if claims == nil {
				writeJSONError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			hasActive, err := subService.HasActiveSubscription(r.Context(), claims.UserID)
			if err != nil {
				writeJSONError(w, http.StatusInternalServerError, "failed to verify subscription status")
				return
			}

			if !hasActive {
				writeJSONError(w, http.StatusForbidden, "active subscription required to access this data")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// NewHumaAuthMiddleware returns an operation middleware for Huma that validates Bearer JWT.
func NewHumaAuthMiddleware(api huma.API, authService service.AuthService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		authHeader := ctx.Header("Authorization")
		if authHeader == "" {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid authorization format, expected 'Bearer <token>'")
			return
		}

		claims, err := authService.ValidateAccessToken(parts[1])
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		ctx = huma.WithValue(ctx, authUserKey, claims)
		next(ctx)
	}
}

// NewHumaRequireSubscriptionMiddleware returns a Huma middleware enforcing active subscription check via Redis.
func NewHumaRequireSubscriptionMiddleware(api huma.API, subService service.SubscriptionService) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		claims := GetAuthUser(ctx.Context())
		if claims == nil {
			_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "authentication required")
			return
		}

		hasActive, err := subService.HasActiveSubscription(ctx.Context(), claims.UserID)
		if err != nil {
			_ = huma.WriteErr(api, ctx, http.StatusInternalServerError, "failed to verify subscription status")
			return
		}

		if !hasActive {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden, "active subscription required to access this resource")
			return
		}

		next(ctx)
	}
}

// GetAuthUser retrieves authenticated user claims from context.
func GetAuthUser(ctx context.Context) *service.JWTClaims {
	if val, ok := ctx.Value(authUserKey).(*service.JWTClaims); ok {
		return val
	}
	return nil
}

// GetUserID retrieves the authenticated user's ID from context.
func GetUserID(ctx context.Context) int64 {
	claims := GetAuthUser(ctx)
	if claims != nil {
		return claims.UserID
	}
	return 0
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": message,
			"code":    status,
		},
	})
}
