package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// statusResponseWriter wraps http.ResponseWriter to capture the HTTP status code and response size.
type statusResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (w *statusResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Logger creates a structured request logger middleware using standard library log/slog.
//
// Node comparison:
// Similar to express-pino-logger or morgan, but logs JSON/text structured fields natively.
func Logger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &statusResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			reqID := GetRequestID(r.Context())

			attrs := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.Int("status", wrapped.statusCode),
				slog.Duration("duration", duration),
				slog.Int("bytes", wrapped.bytes),
			}
			if reqID != "" {
				attrs = append(attrs, slog.String("request_id", reqID))
			}

			if wrapped.statusCode >= 500 {
				logger.ErrorContext(r.Context(), "http request error", attrs...)
			} else {
				logger.InfoContext(r.Context(), "http request completed", attrs...)
			}
		})
	}
}
