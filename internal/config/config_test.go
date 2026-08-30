package config

import (
	"strings"
	"testing"
)

func TestConfig_Validation(t *testing.T) {
	// Dev config should pass with default test encryption key
	devCfg := LoadConfig()
	devCfg.Env = "development"
	devCfg.EncryptionKey = "trueline_aes_key_32_bytes_long!!"
	if err := devCfg.Validate(); err != nil {
		t.Errorf("dev config should be valid, got: %v", err)
	}

	// Invalid encryption key length
	invalidKeyCfg := LoadConfig()
	invalidKeyCfg.EncryptionKey = "short_key"
	if err := invalidKeyCfg.Validate(); err == nil {
		t.Errorf("expected validation failure for short encryption key, got nil")
	}

	// Production with missing secrets should fail
	prodCfg := LoadConfig()
	prodCfg.Env = "production"
	prodCfg.DatabaseURL = "postgres://postgres:postgres@localhost:5432/trueline"
	if err := prodCfg.Validate(); err == nil {
		t.Errorf("expected production validation failure for localhost DB and missing Cashfree/Zego secrets, got nil")
	}
}

func TestConfig_ProductionRejectsMockOTP(t *testing.T) {
	cfg := &Config{
		Env:                  "production",
		DatabaseURL:          "postgres://db.example.test/trueline?sslmode=require",
		JWTSecret:            "a-secure-production-jwt-secret",
		ZegoAppID:            "987654321",
		ZegoServerSecret:     "a-secure-zego-server-secret",
		CashfreeClientID:     "cashfree-client",
		CashfreeClientSecret: "cashfree-secret",
		CashfreeWebhookKey:   "cashfree-webhook-secret",
		OTPMockMode:          true,
		EncryptionKey:        "12345678901234567890123456789012",
		HMACKey:              "a-secure-hmac-key",
	}

	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "OTP_MOCK_MODE") {
		t.Fatalf("expected production mock OTP mode to be rejected, got %v", err)
	}
}
