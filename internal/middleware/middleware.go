// Package middleware provides HTTP middlewares for logging, error recovery, request tracing, and CORS.
//
// Node comparison:
// In Express, middleware is `(req, res, next) => next()`.
// In Go net/http, standard middleware is a function that wraps an `http.Handler`:
// `func(next http.Handler) http.Handler`
package middleware

import (
	"context"
	"net/http"
)

// Middleware is an idiomatic HTTP middleware function signature.
type Middleware func(http.Handler) http.Handler

// Chain executes middlewares in order: the first middleware in the slice
// wraps all subsequent ones and executes first on incoming requests.
func Chain(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// contextKey defines a private type for context keys to prevent cross-package collisions.
// In Go, using plain strings as context keys can cause collisions between libraries.
type contextKey string

const (
	requestIDKey contextKey = "requestID"
)

// GetRequestID extracts the request ID stored in the context by RequestID middleware.
func GetRequestID(ctx context.Context) string {
	if val, ok := ctx.Value(requestIDKey).(string); ok {
		return val
	}
	return ""
}
