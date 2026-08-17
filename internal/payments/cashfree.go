package payments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type RechargePack struct {
	ID          string `json:"id"`
	AmountINR   int64  `json:"amount_inr"`
	AmountPaise int64  `json:"amount_paise"`
	Coins       int64  `json:"coins"`
	CoinsMicros int64  `json:"coins_micros"`
}

var RechargeCatalogue = map[string]RechargePack{
	"pack_49": {
		ID:          "pack_49",
		AmountINR:   49,
		AmountPaise: 4900,
		Coins:       130,
		CoinsMicros: 130_000_000,
	},
	"pack_99": {
		ID:          "pack_99",
		AmountINR:   99,
		AmountPaise: 9900,
		Coins:       260,
		CoinsMicros: 260_000_000,
	},
	"pack_199": {
		ID:          "pack_199",
		AmountINR:   199,
		AmountPaise: 19900,
		Coins:       530,
		CoinsMicros: 530_000_000,
	},
}

type CashfreePGClient struct {
	clientID     string
	clientSecret string
	webhookKey   string
	isSandbox    bool
	httpClient   *http.Client
}

func NewCashfreePGClient(clientID, clientSecret, webhookKey string, isSandbox bool) *CashfreePGClient {
	return &CashfreePGClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		webhookKey:   webhookKey,
		isSandbox:    isSandbox,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *CashfreePGClient) getBaseURL() string {
	if c.isSandbox {
		return "https://sandbox.cashfree.com/pg"
	}
	return "https://api.cashfree.com/pg"
}

type CreateOrderRequest struct {
	OrderID       string  `json:"order_id"`
	OrderAmount   float64 `json:"order_amount"`
	OrderCurrency string  `json:"order_currency"`
	CustomerDetails struct {
		CustomerID    string `json:"customer_id"`
		CustomerPhone string `json:"customer_phone"`
		CustomerEmail string `json:"customer_email,omitempty"`
	} `json:"customer_details"`
	OrderMeta struct {
		ReturnURL      string `json:"return_url,omitempty"`
		NotifyURL      string `json:"notify_url,omitempty"`
		PaymentMethods string `json:"payment_methods,omitempty"`
	} `json:"order_meta"`
}

type CreateOrderResponse struct {
	OrderID          string `json:"order_id"`
	PaymentSessionID string `json:"payment_session_id"`
	OrderStatus      string `json:"order_status"`
	OrderAmount      float64 `json:"order_amount"`
	OrderCurrency    string `json:"order_currency"`
}

func (c *CashfreePGClient) CreateOrder(ctx context.Context, orderID, customerID, customerPhone string, amountINR float64) (*CreateOrderResponse, error) {
	if c.clientID == "" || c.clientSecret == "" {
		// Mock fallback for local offline testing when keys are omitted
		return &CreateOrderResponse{
			OrderID:          orderID,
			PaymentSessionID: fmt.Sprintf("session_mock_%s", orderID),
			OrderStatus:      "ACTIVE",
			OrderAmount:      amountINR,
			OrderCurrency:    "INR",
		}, nil
	}

	endpoint := fmt.Sprintf("%s/orders", c.getBaseURL())

	reqBody := CreateOrderRequest{
		OrderID:       orderID,
		OrderAmount:   amountINR,
		OrderCurrency: "INR",
	}
	reqBody.CustomerDetails.CustomerID = customerID
	reqBody.CustomerDetails.CustomerPhone = customerPhone
	if reqBody.CustomerDetails.CustomerPhone == "" {
		reqBody.CustomerDetails.CustomerPhone = "9999999999"
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal order request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-version", "2023-08-01")
	req.Header.Set("x-client-id", c.clientID)
	req.Header.Set("x-client-secret", c.clientSecret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cashfree order request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read cashfree order response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cashfree order api error (status %d): %s", resp.StatusCode, string(respBytes))
	}

	var orderResp CreateOrderResponse
	if err := json.Unmarshal(respBytes, &orderResp); err != nil {
		return nil, fmt.Errorf("failed to decode cashfree response: %w", err)
	}

	return &orderResp, nil
}

func (c *CashfreePGClient) VerifyWebhookSignature(signature, timestamp string, rawBody []byte) error {
	if c.webhookKey == "" && c.clientSecret == "" {
		return nil // Permitted in local dev when no keys configured
	}

	key := c.webhookKey
	if key == "" {
		key = c.clientSecret
	}

	payload := append([]byte(timestamp), rawBody...)
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write(payload)
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return errors.New("invalid webhook signature")
	}

	return nil
}
