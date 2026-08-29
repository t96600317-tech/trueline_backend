package httpx

import (
	"log"
	"net/http"
	"strings"
	"time"

	"trueline-backend/internal/auth"
)

// LoggingResponseWriter captures the HTTP status code
type LoggingResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

func (lrw *LoggingResponseWriter) WriteHeader(code int) {
	lrw.StatusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// RequestLoggerMiddleware logs every incoming HTTP request with timing and status code
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &LoggingResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("[HTTP] %d | %-6s %s | %v | Client: %s",
			lrw.StatusCode,
			r.Method,
			r.URL.Path,
			duration.Round(time.Millisecond),
			r.RemoteAddr,
		)
	})
}

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, x-webhook-signature, x-webhook-timestamp")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(tm *auth.TokenManager) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				Error(w, http.StatusUnauthorized, "INVALID_HEADER", "Authorization header format must be 'Bearer <token>'")
				return
			}

			tokenStr := parts[1]
			claims, err := tm.ValidateToken(tokenStr)
			if err != nil {
				Error(w, http.StatusUnauthorized, "INVALID_TOKEN", err.Error())
				return
			}

			ctx := auth.ContextWithClaims(r.Context(), claims)
			next(w, r.WithContext(ctx))
		}
	}
}

func OptionalAuthMiddleware(tm *auth.TokenManager) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
					claims, err := tm.ValidateToken(parts[1])
					if err == nil && claims != nil {
						ctx := auth.ContextWithClaims(r.Context(), claims)
						next(w, r.WithContext(ctx))
						return
					}
				}
			}
			next(w, r)
		}
	}
}

func RequireRole(role string) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			allowed := claims.Role == role || claims.Role == "admin" || claims.Role == "superadmin"
			if (role == "listener" || role == "partner") && (claims.Role == "listener" || claims.Role == "partner") {
				allowed = true
			}
			// Allow authenticated user tokens to complete listener onboarding flow
			if (role == "listener" || role == "partner") && strings.Contains(r.URL.Path, "/onboarding") {
				allowed = true
			}

			if !allowed {
				Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource")
				return
			}

			next(w, r)
		}
	}
}
