package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"trueline-backend/internal/config"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	secret := "test_secret_key_12345"
	tm := NewTokenManager(secret)

	userID := uuid.New()
	role := "user"
	phone := "+919876543210"

	token, err := tm.GenerateToken(userID, role, phone, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.Role != role {
		t.Errorf("Expected Role %s, got %s", role, claims.Role)
	}
	if claims.Phone != phone {
		t.Errorf("Expected Phone %s, got %s", phone, claims.Phone)
	}
}

func TestMSG91WidgetOTPVerifier_VerifiesRequest(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"success","message":"OTP verified"}`))
	}))
	defer server.Close()

	verifier := NewMSG91WidgetOTPVerifier("widget-id", "mobile-token")
	verifier.Endpoint = server.URL
	verifier.HTTPClient = server.Client()

	if err := verifier.VerifyOTP(context.Background(), "request-id", "123456"); err != nil {
		t.Fatalf("expected verification to pass, got %v", err)
	}
	if received["widgetId"] != "widget-id" || received["tokenAuth"] != "mobile-token" ||
		received["reqId"] != "request-id" || received["otp"] != "123456" {
		t.Errorf("unexpected MSG91 request: %#v", received)
	}
}

func TestMSG91WidgetOTPVerifier_RejectsErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"error","message":"Incorrect OTP"}`))
	}))
	defer server.Close()

	verifier := NewMSG91WidgetOTPVerifier("widget-id", "mobile-token")
	verifier.Endpoint = server.URL
	verifier.HTTPClient = server.Client()

	if err := verifier.VerifyOTP(context.Background(), "request-id", "000000"); err == nil {
		t.Fatal("expected MSG91 error response to reject the OTP")
	}
}

func TestMSG91WidgetAccessTokenVerifier_VerifiesTokenAndPhone(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"success","message":"Access token verified"}`))
	}))
	defer server.Close()

	accessToken := testMSG91AccessToken(t, map[string]string{"mobile": "919876543210"})
	verifier := NewMSG91WidgetAccessTokenVerifier("server-auth-key")
	verifier.Endpoint = server.URL
	verifier.HTTPClient = server.Client()

	if err := verifier.VerifyAccessToken(context.Background(), accessToken, "+919876543210"); err != nil {
		t.Fatalf("expected verification to pass, got %v", err)
	}
	if received["authkey"] != "server-auth-key" || received["access-token"] != accessToken {
		t.Errorf("unexpected MSG91 request: %#v", received)
	}
}

func TestMSG91WidgetAccessTokenVerifier_RejectsDifferentPhone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"type":"success"}`))
	}))
	defer server.Close()

	verifier := NewMSG91WidgetAccessTokenVerifier("server-auth-key")
	verifier.Endpoint = server.URL
	verifier.HTTPClient = server.Client()
	accessToken := testMSG91AccessToken(t, map[string]string{"identifier": "919876543210"})

	if err := verifier.VerifyAccessToken(context.Background(), accessToken, "912345678901"); err == nil {
		t.Fatal("expected a token for another phone number to be rejected")
	}
}

func TestAuthService_RejectsAccessTokenWhenVerifierIsMissing(t *testing.T) {
	service := NewAuthService(nil, NewTokenManager("test-secret"), NewMockOTPProvider(), &config.Config{
		OTPMockMode:   false,
		EncryptionKey: "12345678901234567890123456789012",
		HMACKey:       "test-hmac-key",
	})

	_, err := service.VerifyOTP(context.Background(), "+919876543210", "123456", "user", "request-id", "access-token")
	if err == nil || !strings.Contains(err.Error(), "access-token verification is not configured") {
		t.Fatalf("expected a missing access-token verifier error, got %v", err)
	}
}

func testMSG91AccessToken(t *testing.T, claims map[string]string) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	return "header." + base64.RawURLEncoding.EncodeToString(payload) + ".signature"
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !CheckPasswordHash(password, hash) {
		t.Errorf("Password hash check failed for correct password")
	}

	if CheckPasswordHash("WrongPass", hash) {
		t.Errorf("Password hash check succeeded for incorrect password")
	}
}
