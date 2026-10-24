package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// RequestID generates a unique identifier for each incoming HTTP request if one is not provided.
// It sets the ID in the response header and attaches it to request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = generateRandomID()
		}

		// Echo the request ID back to the client
		w.Header().Set("X-Request-ID", reqID)

		// Inject into context so downstream handlers, services, and loggers can access it
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// generateRandomID produces a 16-byte cryptographically secure hexadecimal string.
// Zero third-party dependency: uses Go standard library crypto/rand.
func generateRandomID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
