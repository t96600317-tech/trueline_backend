package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                 string
	Env                  string
	DatabaseURL          string
	SupabaseURL          string
	SupabaseAnonKey      string
	SupabaseServiceKey   string
	SupabaseBucket       string
	JWTSecret            string
	ZegoAppID            string
	ZegoServerSecret     string
	RazorpayKeyID        string
	RazorpayKeySecret    string
	OTPProvider          string // "mock", "twilio", "msg91"
	OTPMockMode          bool
	TwilioAccountSID     string
	TwilioAuthToken      string
	TwilioFromPhone      string
	MSG91AuthKey         string
	MSG91TemplateID      string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	cfg := &Config{
		Port:                 getEnv("PORT", "8080"),
		Env:                  getEnv("ENV", "development"),
		DatabaseURL:          getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/trueline?sslmode=disable"),
		SupabaseURL:          getEnv("SUPABASE_URL", ""),
		SupabaseAnonKey:      getEnv("SUPABASE_ANON_KEY", ""),
		SupabaseServiceKey:   getEnv("SUPABASE_SERVICE_ROLE_KEY", ""),
		SupabaseBucket:       getEnv("SUPABASE_STORAGE_BUCKET", "kyc-documents"),
		JWTSecret:            getEnv("JWT_SECRET", "trueline_default_jwt_secret_change_in_prod"),
		ZegoAppID:            getEnv("ZEGO_APP_ID", "123456789"),
		ZegoServerSecret:     getEnv("ZEGO_SERVER_SECRET", "default_zego_secret"),
		RazorpayKeyID:        getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret:    getEnv("RAZORPAY_KEY_SECRET", ""),
		OTPProvider:          getEnv("OTP_PROVIDER", "mock"),
		OTPMockMode:          getEnv("OTP_MOCK_MODE", "true") == "true",
		TwilioAccountSID:     getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:      getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromPhone:      getEnv("TWILIO_FROM_PHONE", ""),
		MSG91AuthKey:         getEnv("MSG91_AUTH_KEY", ""),
		MSG91TemplateID:      getEnv("MSG91_TEMPLATE_ID", ""),
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}
