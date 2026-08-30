package config

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                         string
	Env                          string
	DatabaseURL                  string
	SupabaseURL                  string
	SupabaseAnonKey              string
	SupabaseServiceKey           string
	SupabaseBucket               string
	JWTSecret                    string
	ZegoAppID                    string
	ZegoServerSecret             string
	CashfreeClientID             string
	CashfreeClientSecret         string
	CashfreeWebhookKey           string
	CashfreeSandbox              bool
	OTPProvider                  string // "mock", "twilio", "msg91"
	OTPMockMode                  bool
	TwilioAccountSID             string
	TwilioAuthToken              string
	TwilioFromPhone              string
	MSG91AuthKey                 string
	MSG91TemplateID              string
	MSG91WidgetID                string
	MSG91WidgetAuthToken         string
	MSG91ServerAuthKey           string
	MSG91CustomerWidgetID        string
	MSG91CustomerWidgetAuthToken string
	MSG91CustomerServerAuthKey   string
	MSG91ListenerWidgetID        string
	MSG91ListenerWidgetAuthToken string
	MSG91ListenerServerAuthKey   string
	EncryptionKey                string
	HMACKey                      string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	encKey := getEnv("ENCRYPTION_KEY", "trueline_aes_key_32_bytes_long!!")
	// If hex encoded (64 chars), decode to 32 bytes string
	if len(encKey) == 64 {
		if decoded, err := hex.DecodeString(encKey); err == nil && len(decoded) == 32 {
			encKey = string(decoded)
		}
	}
	// In development, ensure it is 32 bytes so server boots reliably
	if len(encKey) != 32 {
		if len(encKey) < 32 {
			encKey = (encKey + "01234567890123456789012345678901")[:32]
		} else {
			encKey = encKey[:32]
		}
	}

	cfg := &Config{
		Port:                         getEnv("PORT", "8080"),
		Env:                          getEnv("ENV", "development"),
		DatabaseURL:                  getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trueline?sslmode=disable"),
		SupabaseURL:                  getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:              getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceKey:           getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
		SupabaseBucket:               getEnv("SUPABASE_STORAGE_BUCKET", "kyc-documents"),
		JWTSecret:                    getEnv("JWT_SECRET", "trueline_default_jwt_secret_change_in_prod"),
		ZegoAppID:                    getEnv("ZEGO_APP_ID", "123456789"),
		ZegoServerSecret:             getEnv("ZEGO_SERVER_SECRET", "default_zego_secret"),
		CashfreeClientID:             getEnv("CASHFREE_CLIENT_ID", ""),
		CashfreeClientSecret:         getEnv("CASHFREE_CLIENT_SECRET", ""),
		CashfreeWebhookKey:           getEnv("CASHFREE_WEBHOOK_KEY", ""),
		CashfreeSandbox:              getEnv("CASHFREE_SANDBOX", "true") == "true",
		OTPProvider:                  getEnv("OTP_PROVIDER", "mock"),
		OTPMockMode:                  getEnv("OTP_MOCK_MODE", "true") == "true",
		TwilioAccountSID:             getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:              getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromPhone:              getEnv("TWILIO_FROM_PHONE", ""),
		MSG91AuthKey:                 getEnv("MSG91_AUTH_KEY", ""),
		MSG91TemplateID:              getEnv("MSG91_TEMPLATE_ID", ""),
		MSG91WidgetID:                getEnv("MSG91_WIDGET_ID", ""),
		MSG91WidgetAuthToken:         getEnv("MSG91_WIDGET_AUTH_TOKEN", ""),
		MSG91ServerAuthKey:           getEnv("MSG91_SERVER_AUTH_KEY", ""),
		MSG91CustomerWidgetID:        getEnv("MSG91_CUSTOMER_WIDGET_ID", ""),
		MSG91CustomerWidgetAuthToken: getEnv("MSG91_CUSTOMER_WIDGET_AUTH_TOKEN", ""),
		MSG91CustomerServerAuthKey:   getEnv("MSG91_CUSTOMER_SERVER_AUTH_KEY", ""),
		MSG91ListenerWidgetID:        getEnv("MSG91_LISTENER_WIDGET_ID", ""),
		MSG91ListenerWidgetAuthToken: getEnv("MSG91_LISTENER_WIDGET_AUTH_TOKEN", ""),
		MSG91ListenerServerAuthKey:   getEnv("MSG91_LISTENER_SERVER_AUTH_KEY", ""),
		EncryptionKey:                encKey,
		HMACKey:                      getEnv("HMAC_KEY", "trueline_hmac_key_32_bytes_long!"),
	}

	return cfg
}

func (c *Config) Validate() error {
	if c.Env == "production" || c.Env == "staging" {
		var missing []string
		if c.OTPMockMode {
			missing = append(missing, "OTP_MOCK_MODE must be false outside development")
		}

		if c.DatabaseURL == "" || strings.Contains(c.DatabaseURL, "localhost") {
			missing = append(missing, "DATABASE_URL (must not be empty or localhost)")
		}
		if c.JWTSecret == "" || c.JWTSecret == "trueline_default_jwt_secret_change_in_prod" {
			missing = append(missing, "JWT_SECRET (must be configured securely)")
		}
		if len(c.EncryptionKey) != 32 || c.EncryptionKey == "default_encryption_key_32_bytes_long!!" {
			missing = append(missing, "ENCRYPTION_KEY (must be exactly 32 bytes and not default)")
		}
		if c.HMACKey == "" || c.HMACKey == "default_hmac_key_for_pilot_v1_!!!" {
			missing = append(missing, "HMAC_KEY (must be configured securely)")
		}
		if c.CashfreeClientID == "" {
			missing = append(missing, "CASHFREE_CLIENT_ID")
		}
		if c.CashfreeClientSecret == "" {
			missing = append(missing, "CASHFREE_CLIENT_SECRET")
		}
		if c.CashfreeWebhookKey == "" {
			missing = append(missing, "CASHFREE_WEBHOOK_KEY")
		}
		if c.ZegoAppID == "" || c.ZegoAppID == "123456789" {
			missing = append(missing, "ZEGO_APP_ID")
		}
		if c.ZegoServerSecret == "" || c.ZegoServerSecret == "default_zego_secret" {
			missing = append(missing, "ZEGO_SERVER_SECRET")
		}
		if (c.MSG91WidgetID == "") != (c.MSG91WidgetAuthToken == "") {
			missing = append(missing, "MSG91_WIDGET_ID and MSG91_WIDGET_AUTH_TOKEN (must be configured together)")
		}
		if (c.MSG91CustomerWidgetID == "") != (c.MSG91CustomerWidgetAuthToken == "") {
			missing = append(missing, "MSG91_CUSTOMER_WIDGET_ID and MSG91_CUSTOMER_WIDGET_AUTH_TOKEN (must be configured together)")
		}
		if (c.MSG91ListenerWidgetID == "") != (c.MSG91ListenerWidgetAuthToken == "") {
			missing = append(missing, "MSG91_LISTENER_WIDGET_ID and MSG91_LISTENER_WIDGET_AUTH_TOKEN (must be configured together)")
		}

		if len(missing) > 0 {
			return fmt.Errorf("missing or invalid production configuration secrets: %s", strings.Join(missing, ", "))
		}
	}

	if len(c.EncryptionKey) != 32 {
		return errors.New("ENCRYPTION_KEY must be exactly 32 bytes (256 bits) for AES-256-GCM")
	}

	return nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}
