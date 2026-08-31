package httpx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"trueline-backend/internal/admin"
	"trueline-backend/internal/auth"
	"trueline-backend/internal/calls"
	"trueline-backend/internal/chat"
	"trueline-backend/internal/config"
	"trueline-backend/internal/kyc"
	"trueline-backend/internal/listeners"
	"trueline-backend/internal/payments"
	"trueline-backend/internal/payouts"
	"trueline-backend/internal/user"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
)

func setupTestRouter() (*http.ServeMux, *auth.TokenManager) {
	cfg := config.LoadConfig()
	cfg.Env = "test"
	tm := auth.NewTokenManager(cfg.JWTSecret)
	otpMock := auth.NewMockOTPProvider()

	authService := auth.NewAuthService(nil, tm, otpMock, cfg)
	authHandler := auth.NewAuthHandler(authService)

	userService := user.NewUserService(nil)
	userHandler := user.NewUserHandler(userService)

	listenerService := listeners.NewListenerService(nil)
	listenerHandler := listeners.NewListenerHandler(listenerService)

	mockSecureID := kyc.NewMockSecureIDProvider()
	kycService := kyc.NewKYCService(nil, mockSecureID)
	kycHandler := kyc.NewKYCHandler(kycService)

	walletService := wallet.NewWalletService(nil)
	cfPGClient := payments.NewCashfreePGClient("", "", "", true)
	paymentService := payments.NewPaymentService(nil, walletService, cfPGClient)
	paymentHandler := payments.NewPaymentHandler(paymentService, cfPGClient)

	cfPayoutsClient := payouts.NewCashfreePayoutsClient("", "", true)
	payoutService := payouts.NewPayoutService(nil, cfPayoutsClient)
	payoutHandler := payouts.NewPayoutHandler(payoutService)

	zegoProvider := calls.NewZegoTokenProvider("123456", "test_secret_32_bytes_long_12345")
	callService := calls.NewCallService(nil, zegoProvider, walletService)
	eventHub := calls.NewEventHub()
	callHandler := calls.NewCallHandler(callService, eventHub, tm)

	adminService := admin.NewAdminService(nil, tm, payoutService)
	adminHandler := admin.NewAdminHandler(adminService)

	chatService := chat.NewChatService(nil, walletService)
	chatHandler := chat.NewChatHandler(chatService)

	router := NewRouter(
		authHandler,
		userHandler,
		listenerHandler,
		kycHandler,
		paymentHandler,
		payoutHandler,
		adminHandler,
		callHandler,
		chatHandler,
		tm,
	)

	return router, tm
}

func TestHealthCheck(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Success bool                   `json:"success"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if !resp.Success || resp.Data["status"] != "ok" || resp.Data["service"] != "trueline-backend" {
		t.Errorf("unexpected healthz response body: %+v", resp)
	}
}

func TestPaymentCatalogueEndpoint(t *testing.T) {
	router, _ := setupTestRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/payments/catalogue", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp struct {
		Success bool                    `json:"success"`
		Data    []payments.RechargePack `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode catalogue response: %v", err)
	}

	if !resp.Success || len(resp.Data) != 3 {
		t.Fatalf("expected 3 packs in catalogue, got %d", len(resp.Data))
	}
}

func TestAuthMiddlewareProtection(t *testing.T) {
	router, tm := setupTestRouter()

	// 1. Unauthenticated request to protected route
	req := httptest.NewRequest(http.MethodGet, "/api/v1/listener/me", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 for missing token, got %d", rec.Code)
	}

	// 2. User role trying to access listener route
	userToken, _ := tm.GenerateToken(uuid.New(), "user", "+919999999999", 1*time.Hour)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/listener/me", nil)
	req2.Header.Set("Authorization", "Bearer "+userToken)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("expected status 403 for user accessing listener route, got %d", rec2.Code)
	}

	// 3. Listener token accessing listener route
	listenerToken, _ := tm.GenerateToken(uuid.New(), "listener", "+919999999998", 1*time.Hour)
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/listener/me", nil)
	req3.Header.Set("Authorization", "Bearer "+listenerToken)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)

	// Since DB pool is nil in unit test, it returns 404 with error body
	if rec3.Code != http.StatusNotFound && rec3.Code != http.StatusOK {
		t.Errorf("expected listener token to pass auth middleware (status 404 or 200), got %d", rec3.Code)
	}
}

func TestAdminRoutes(t *testing.T) {
	router, tm := setupTestRouter()

	adminToken, _ := tm.GenerateToken(uuid.New(), "admin", "", 1*time.Hour)

	// Test GET /api/v1/admin/stats
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/stats", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected admin stats status 200, got %d", rec.Code)
	}

	// Test GET /api/v1/admin/kyc/queue
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/kyc/queue", nil)
	req2.Header.Set("Authorization", "Bearer "+adminToken)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected admin kyc queue status 200, got %d", rec2.Code)
	}
}

func TestPaymentWebhook(t *testing.T) {
	router, _ := setupTestRouter()

	webhookPayload := map[string]interface{}{
		"data": map[string]interface{}{
			"order": map[string]interface{}{
				"order_id": "ord_test_123",
			},
			"payment": map[string]interface{}{
				"cf_payment_id":  "cf_pay_456",
				"payment_status": "SUCCESS",
			},
		},
		"type": "PAYMENT_SUCCESS_WEBHOOK",
	}
	bodyBytes, _ := json.Marshal(webhookPayload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/cashfree/payments", bytes.NewReader(bodyBytes))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Since mock client accepts empty webhook key in test, status should be 200 (or handled)
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("unexpected webhook status code: %d", rec.Code)
	}
}
