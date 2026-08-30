package httpx

import (
	"net/http"

	"trueline-backend/internal/admin"
	"trueline-backend/internal/auth"
	"trueline-backend/internal/calls"
	"trueline-backend/internal/chat"
	"trueline-backend/internal/kyc"
	"trueline-backend/internal/listeners"
	"trueline-backend/internal/payments"
	"trueline-backend/internal/payouts"
	"trueline-backend/internal/user"
)

func NewRouter(
	authHandler *auth.AuthHandler,
	userHandler *user.UserHandler,
	listenerHandler *listeners.ListenerHandler,
	kycHandler *kyc.KYCHandler,
	paymentHandler *payments.PaymentHandler,
	payoutHandler *payouts.PayoutHandler,
	adminHandler *admin.AdminHandler,
	callHandler *calls.CallHandler,
	chatHandler *chat.ChatHandler,
	tm *auth.TokenManager,
) *http.ServeMux {
	mux := http.NewServeMux()

	authMiddleware := AuthMiddleware(tm)
	userRoleMiddleware := RequireRole("user")
	listenerRoleMiddleware := RequireRole("listener")
	adminRoleMiddleware := RequireRole("admin")

	// Health Check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "trueline-backend", "version": "pilot-v1"})
	})

	// 1. Auth Endpoints (Public)
	mux.HandleFunc("POST /api/v1/auth/otp/request", authHandler.RequestOTP)
	mux.HandleFunc("POST /api/v1/auth/otp/verify", authHandler.VerifyOTP)

	// 2. User App Endpoints
	mux.HandleFunc("GET /api/v1/user/me", Chain(userHandler.GetMe, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/user/heartbeat", Chain(userHandler.Heartbeat, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("PATCH /api/v1/user/profile", Chain(userHandler.UpdateProfile, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/user/profile", Chain(userHandler.UpdateProfile, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("PATCH /api/v1/user/language", Chain(userHandler.UpdateLanguage, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listeners", Chain(userHandler.DiscoverListeners, OptionalAuthMiddleware(tm)))
	mux.HandleFunc("POST /api/v1/listeners/{id}/notify-me", Chain(listenerHandler.NotifyMe, authMiddleware, userRoleMiddleware))

	// 3. Listener App Endpoints
	mux.HandleFunc("GET /api/v1/listener/me", Chain(listenerHandler.GetMe, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("PATCH /api/v1/listener/onboarding/profile", Chain(listenerHandler.UpdateOnboardingProfile, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/voice", Chain(listenerHandler.UpdateVoiceIntro, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/kyc/pan", Chain(kycHandler.SubmitPAN, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/kyc/bank", Chain(kycHandler.SubmitBank, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/kyc/selfie", Chain(kycHandler.SubmitSelfie, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/kyc/agreement", Chain(kycHandler.SubmitAgreement, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/onboarding/submit", Chain(listenerHandler.SubmitOnboarding, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listner/onboarding/submit", Chain(listenerHandler.SubmitOnboarding, authMiddleware, listenerRoleMiddleware)) // Alias
	mux.HandleFunc("POST /api/v1/listener/availability", Chain(listenerHandler.SetAvailability, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/home", Chain(listenerHandler.GetHomeDashboard, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/milestones", Chain(listenerHandler.GetMilestones, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/score", Chain(listenerHandler.GetPerformanceScore, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/detailed-earnings", Chain(listenerHandler.GetDetailedEarnings, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/withdraw", Chain(listenerHandler.RequestWithdrawal, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/blocked-users", Chain(listenerHandler.GetBlockedUsers, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/reports", Chain(listenerHandler.SubmitReport, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/earnings", Chain(payoutHandler.GetEarnings, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/listener/payout-requests", Chain(payoutHandler.RequestPayout, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/call-history", Chain(listenerHandler.GetCallHistory, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/calls", Chain(listenerHandler.GetCallHistory, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/transactions", Chain(listenerHandler.GetTransactions, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("GET /api/v1/listener/notifications", Chain(listenerHandler.GetNotifications, authMiddleware, listenerRoleMiddleware))

	// 4. Payment Endpoints
	mux.HandleFunc("GET /api/v1/payments/catalogue", paymentHandler.GetCatalogue)
	mux.HandleFunc("POST /api/v1/user/recharge", Chain(paymentHandler.InitiateRecharge, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/payments/create-order", Chain(paymentHandler.CreateOrder, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("GET /api/v1/payments/orders/{id}/verify", Chain(paymentHandler.VerifyOrder, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/webhooks/cashfree/payments", paymentHandler.CashfreeWebhook)

	// 5. Calling Endpoints
	mux.HandleFunc("GET /api/v1/listener/calls/incoming", Chain(callHandler.GetIncomingCall, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/calls", Chain(callHandler.InitiateCall, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/calls/initiate", Chain(callHandler.InitiateCall, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("POST /api/v1/calls/{id}/accept", Chain(callHandler.AcceptCall, authMiddleware, listenerRoleMiddleware))
	mux.HandleFunc("POST /api/v1/calls/{id}/end", Chain(callHandler.EndCall, authMiddleware))
	mux.HandleFunc("GET /api/v1/calls/{id}/summary", Chain(callHandler.GetCallSummary, authMiddleware))
	mux.HandleFunc("POST /api/v1/calls/{id}/rate", Chain(callHandler.RateCall, authMiddleware, userRoleMiddleware))
	mux.HandleFunc("GET /api/v1/calls/{id}/events", callHandler.HandleCallEventsWS)

	// 6. Admin Endpoints
	mux.HandleFunc("POST /api/v1/admin/login", adminHandler.Login)
	mux.HandleFunc("GET /api/v1/admin/stats", Chain(adminHandler.GetStats, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("GET /api/v1/admin/kyc/queue", Chain(adminHandler.ListKYCQueue, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("POST /api/v1/admin/kyc/{id}/review", Chain(adminHandler.ReviewKYC, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("GET /api/v1/admin/listeners", Chain(adminHandler.ListListeners, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("GET /api/v1/admin/users", Chain(adminHandler.ListUsers, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("POST /api/v1/admin/listeners/{id}/status", Chain(adminHandler.ToggleListenerStatus, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("GET /api/v1/admin/payouts", Chain(adminHandler.ListPayouts, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("POST /api/v1/admin/payouts/{id}/process", Chain(adminHandler.ProcessPayout, authMiddleware, adminRoleMiddleware))
	mux.HandleFunc("GET /api/v1/admin/ledgers", Chain(adminHandler.ListLedgers, authMiddleware, adminRoleMiddleware))

	// 7. Chat Endpoints
	mux.HandleFunc("GET /api/v1/chats", Chain(chatHandler.ListConversations, authMiddleware))
	mux.HandleFunc("GET /api/v1/chats/{id}/messages", Chain(chatHandler.GetMessages, authMiddleware))
	mux.HandleFunc("POST /api/v1/chats/{id}/messages", Chain(chatHandler.SendMessage, authMiddleware))
	mux.HandleFunc("GET /api/v1/chat/conversations", Chain(chatHandler.ListConversations, authMiddleware))
	mux.HandleFunc("GET /api/v1/chat/conversations/{id}/messages", Chain(chatHandler.GetMessages, authMiddleware))
	mux.HandleFunc("POST /api/v1/chat/conversations/{id}/messages", Chain(chatHandler.SendMessage, authMiddleware))

	return mux
}
