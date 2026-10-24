package middleware

import (
	"net/http"
)

// CORS configures Cross-Origin Resource Sharing headers.
//
// Node comparison:
// Equivalent to `app.use(cors({ origin: '...', methods: [...] }))` in Express.
func CORS(allowedOrigin string) Middleware {
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

			// Handle preflight OPTIONS requests immediately without passing downstream
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
