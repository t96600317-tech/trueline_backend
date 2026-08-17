package config

import (
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
