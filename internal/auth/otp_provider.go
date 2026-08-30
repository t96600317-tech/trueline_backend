package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OTPProvider interface {
	SendOTP(ctx context.Context, phone, otp string) error
}

// WidgetOTPVerifier verifies a code with MSG91 before the backend creates a
// TrueLine session. The app's SDK sends/retries the OTP; verification happens
// here so a modified client cannot mint a JWT by claiming success.
type WidgetOTPVerifier interface {
	VerifyOTP(ctx context.Context, requestID, otp string) error
}

// WidgetAccessTokenVerifier validates the JWT returned by MSG91's Android
// widget after the app verifies an OTP. The server-side auth key never leaves
// the backend.
type WidgetAccessTokenVerifier interface {
	VerifyAccessToken(ctx context.Context, accessToken, expectedPhone string) error
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

// MSG91WidgetOTPVerifier uses the same Widget API as the Android SendOTP SDK
// for server-side verification of a request ID returned by that SDK.
type MSG91WidgetOTPVerifier struct {
	WidgetID   string
	AuthToken  string
	Endpoint   string
	HTTPClient *http.Client
}

func NewMSG91WidgetOTPVerifier(widgetID, authToken string) *MSG91WidgetOTPVerifier {
	return &MSG91WidgetOTPVerifier{
		WidgetID:   widgetID,
		AuthToken:  authToken,
		Endpoint:   "https://control.msg91.com/api/v5/widget/verifyOtp",
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *MSG91WidgetOTPVerifier) VerifyOTP(ctx context.Context, requestID, otp string) error {
	if m.WidgetID == "" || m.AuthToken == "" {
		return fmt.Errorf("MSG91 widget credentials not configured")
	}
	if requestID == "" || otp == "" {
		return fmt.Errorf("MSG91 request ID and OTP are required")
	}

	payload, err := json.Marshal(map[string]string{
		"widgetId":  m.WidgetID,
		"tokenAuth": m.AuthToken,
		"reqId":     requestID,
		"otp":       otp,
	})
	if err != nil {
		return fmt.Errorf("failed to encode MSG91 verification request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call MSG91 widget API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("failed to read MSG91 widget response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("MSG91 widget API returned status %d", resp.StatusCode)
	}

	var result struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("invalid MSG91 widget response: %w", err)
	}
	if strings.EqualFold(result.Type, "error") {
		if result.Message == "" {
			return fmt.Errorf("MSG91 rejected the OTP")
		}
		return fmt.Errorf("MSG91 rejected the OTP: %s", result.Message)
	}

	return nil
}

// MSG91WidgetAccessTokenVerifier verifies the access token produced by
// OTPWidget.verifyOTP through MSG91's server-side API. The token's phone claim
// is compared only after MSG91 has validated the token signature, preventing a
// client from using a valid token for one number to sign in as another.
type MSG91WidgetAccessTokenVerifier struct {
	AuthKey    string
	Endpoint   string
	HTTPClient *http.Client
}

func NewMSG91WidgetAccessTokenVerifier(authKey string) *MSG91WidgetAccessTokenVerifier {
	return &MSG91WidgetAccessTokenVerifier{
		AuthKey:    authKey,
		Endpoint:   "https://control.msg91.com/api/v5/widget/verifyAccessToken",
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *MSG91WidgetAccessTokenVerifier) VerifyAccessToken(ctx context.Context, accessToken, expectedPhone string) error {
	if m.AuthKey == "" {
		return fmt.Errorf("MSG91 server auth key not configured")
	}
	if accessToken == "" {
		return fmt.Errorf("MSG91 access token is required")
	}

	payload, err := json.Marshal(map[string]string{
		"authkey":      m.AuthKey,
		"access-token": accessToken,
	})
	if err != nil {
		return fmt.Errorf("failed to encode MSG91 access-token request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call MSG91 access-token API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return fmt.Errorf("failed to read MSG91 access-token response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("MSG91 access-token API returned status %d", resp.StatusCode)
	}

	var result struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("invalid MSG91 access-token response: %w", err)
	}
	if strings.EqualFold(result.Type, "error") {
		if result.Message == "" {
			return fmt.Errorf("MSG91 rejected the access token")
		}
		return fmt.Errorf("MSG91 rejected the access token: %s", result.Message)
	}

	verifiedPhone, err := msg91PhoneFromVerifiedAccessToken(accessToken)
	if err != nil {
		return err
	}
	if !sameIndianPhone(verifiedPhone, expectedPhone) {
		return fmt.Errorf("MSG91 access token was issued for a different phone number")
	}

	return nil
}

func msg91PhoneFromVerifiedAccessToken(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("MSG91 returned an invalid access token")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("MSG91 returned an invalid access token")
	}

	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var claims map[string]any
	if err := decoder.Decode(&claims); err != nil {
		return "", fmt.Errorf("MSG91 returned an invalid access token")
	}
	for _, key := range []string{"mobile", "mobile_number", "phone", "phone_number", "phoneNumber", "identifier"} {
		if value := msg91StringClaim(claims[key]); value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("MSG91 access token did not include a verified phone number")
}

func msg91StringClaim(value any) string {
	switch value := value.(type) {
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	default:
		return ""
	}
}

func sameIndianPhone(first, second string) bool {
	normalize := func(phone string) string {
		var digits strings.Builder
		for _, char := range phone {
			if char >= '0' && char <= '9' {
				digits.WriteRune(char)
			}
		}
		result := digits.String()
		if len(result) == 10 {
			return "91" + result
		}
		return result
	}

	first = normalize(first)
	second = normalize(second)
	return first != "" && first == second
}
