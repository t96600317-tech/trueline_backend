package httpx

import (
	"net/http"

	"trueline-backend/internal/auth"
	"trueline-backend/internal/chat"
	"trueline-backend/internal/user"
)

func NewRouter(
	authHandler *auth.AuthHandler,
	userHandler *user.UserHandler,
	chatHandler *chat.ChatHandler,
	tm *auth.TokenManager,
) *http.ServeMux {
	mux := http.NewServeMux()

	authMiddleware := AuthMiddleware(tm)
	userRoleMiddleware := RequireRole("user")

	// Health Check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "trueline-backend"})
	})

	// 1. Auth Endpoints (Public)
	mux.HandleFunc("POST /api/v1/auth/otp/request", authHandler.RequestOTP)
	mux.HandleFunc("POST /api/v1/auth/otp/verify", authHandler.VerifyOTP)

	// 2. User Profile & Language Preference
	mux.HandleFunc("GET /api/v1/user/me", Chain(userHandler.GetMe, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("PATCH /api/v1/user/language", Chain(userHandler.UpdateLanguage, authMiddleware, userRoleMiddleware))

	// 3. Home / Discover Partners Listing
	mux.HandleFunc("GET /api/v1/partners", Chain(userHandler.DiscoverPartners, authMiddleware, userRoleMiddleware))

	// 4. Chat Endpoints (User App)
	mux.HandleFunc("GET /api/v1/chats", Chain(chatHandler.ListConversations, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("GET /api/v1/chats/{partner_id}/messages", Chain(chatHandler.GetMessages, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/chats/{partner_id}/messages", Chain(chatHandler.SendMessage, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/chats/{partner_id}/read", Chain(chatHandler.MarkRead, authMiddleware, userRoleMiddleware))

	// 5. Wallet Routes Placeholder
	mux.HandleFunc("GET /api/v1/wallet", Chain(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]interface{}{"balance": 260.00, "transactions": []interface{}{}})
	}, authMiddleware, userRoleMiddleware))

	// 6. Call Routes Placeholder
	mux.HandleFunc("POST /api/v1/calls/initiate", Chain(func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"message": "Call initiated"})
	}, authMiddleware, userRoleMiddleware))

	return mux
}
