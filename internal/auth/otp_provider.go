package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OTPProvider interface {
	SendOTP(ctx context.Context, phone, otp string) error
}

// 1. Mock OTP Provider (Development / Staging)
type MockOTPProvider struct{}

func NewMockOTPProvider() *MockOTPProvider {
	return &MockOTPProvider{}
}

func (m *MockOTPProvider) SendOTP(ctx context.Context, phone, otp string) error {
	log.Printf("[MOCK OTP SMS] Sent OTP code '%s' to phone '%s'", otp, phone)
	return nil
}

// 2. Twilio OTP Provider (Production)
type TwilioOTPProvider struct {
	AccountSID string
	AuthToken  string
	FromPhone  string
	HTTPClient *http.Client
}

func NewTwilioOTPProvider(accountSID, authToken, fromPhone string) *TwilioOTPProvider {
	return &TwilioOTPProvider{
		AccountSID: accountSID,
		AuthToken:  authToken,
		FromPhone:  fromPhone,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *TwilioOTPProvider) SendOTP(ctx context.Context, phone, otp string) error {
	if t.AccountSID == "" || t.AuthToken == "" {
		return fmt.Errorf("twilio credentials not configured")
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", t.AccountSID)
	bodyText := fmt.Sprintf("Your TrueLine verification code is %s. Do not share it with anyone.", otp)

	data := url.Values{}
	data.Set("To", phone)
	data.Set("From", t.FromPhone)
	data.Set("Body", bodyText)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Twilio API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("twilio API returned error status code: %d", resp.StatusCode)
	}

	log.Printf("[TWILIO SMS] Successfully sent OTP to %s", phone)
	return nil
}

// 3. MSG91 OTP Provider (India Market Standard)
type MSG91OTPProvider struct {
	AuthKey    string
	TemplateID string
	HTTPClient *http.Client
}

func NewMSG91OTPProvider(authKey, templateID string) *MSG91OTPProvider {
	return &MSG91OTPProvider{
		AuthKey:    authKey,
		TemplateID: templateID,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *MSG91OTPProvider) SendOTP(ctx context.Context, phone, otp string) error {
	if m.AuthKey == "" {
		return fmt.Errorf("MSG91 auth key not configured")
	}

	// Clean phone number format for India (+91 or 91)
	cleanPhone := strings.TrimPrefix(phone, "+")
	apiURL := fmt.Sprintf("https://api.msg91.com/api/v5/otp?template_id=%s&mobile=%s&otp=%s", m.TemplateID, cleanPhone, otp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return err
	}

	req.Header.Add("authkey", m.AuthKey)

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call MSG91 API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("MSG91 API returned error status code: %d", resp.StatusCode)
	}

	log.Printf("[MSG91 SMS] Successfully sent OTP to %s", phone)
	return nil
}
