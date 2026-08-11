package httpx

import (
	"net/http"
	"strings"

	"trueline-backend/internal/auth"
)

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

func RequireRole(role string) Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			claims, ok := auth.ClaimsFromContext(r.Context())
			if !ok || claims == nil {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
				return
			}

			if claims.Role != role && claims.Role != "admin" && claims.Role != "superadmin" {
				Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to access this resource")
				return
			}

			next(w, r)
		}
	}
}
