package middleware

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recoverer catches unhandled panics, logs the stack trace, and returns a 500 Internal Server Error.
//
// Node comparison:
// In Node.js, an unhandled synchronous exception in an Express route crashes the process unless
// caught by an error middleware `(err, req, res, next)`. In Go, runtime panics are intercepted
// using the built-in `recover()` inside a `defer` block.
func Recoverer(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					stack := string(debug.Stack())
					reqID := GetRequestID(r.Context())

					logger.ErrorContext(r.Context(), "panic recovered",
						slog.Any("error", rec),
						slog.String("request_id", reqID),
						slog.String("stack", stack),
					)

					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)

					_ = json.NewEncoder(w).Encode(map[string]any{
						"error": map[string]any{
							"message": fmt.Sprintf("Internal Server Error: %v", rec),
							"code":    http.StatusInternalServerError,
						},
					})
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
