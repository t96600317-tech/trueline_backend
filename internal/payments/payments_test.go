package payments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
)

func TestPaymentCatalogue(t *testing.T) {
	if len(RechargeCatalogue) != 3 {
		t.Fatalf("expected 3 recharge packs, got %d", len(RechargeCatalogue))
	}

	p49, ok := RechargeCatalogue["pack_49"]
	if !ok || p49.AmountINR != 49 || p49.Coins != 130 {
		t.Errorf("invalid pack_49 definition: %+v", p49)
	}

	p99, ok := RechargeCatalogue["pack_99"]
	if !ok || p99.AmountINR != 99 || p99.Coins != 260 {
		t.Errorf("invalid pack_99 definition: %+v", p99)
	}

	p199, ok := RechargeCatalogue["pack_199"]
	if !ok || p199.AmountINR != 199 || p199.Coins != 530 {
		t.Errorf("invalid pack_199 definition: %+v", p199)
	}
}

func TestCashfreePGClient_WebhookSignature(t *testing.T) {
	secret := "test_webhook_secret_key_12345"
	client := NewCashfreePGClient("test_id", "test_sec", secret, true)

	rawBody := []byte(`{"data":{"order":{"order_id":"ord_123"}},"type":"PAYMENT_SUCCESS_WEBHOOK"}`)
	timestamp := "1723810000"

	payload := append([]byte(timestamp), rawBody...)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if err := client.VerifyWebhookSignature(validSig, timestamp, rawBody); err != nil {
		t.Errorf("expected valid signature to pass, got error: %v", err)
	}

	if err := client.VerifyWebhookSignature("invalid_signature", timestamp, rawBody); err == nil {
		t.Errorf("expected invalid signature to fail, got nil")
	}
}

func TestPaymentService_InvalidPack(t *testing.T) {
	service := NewPaymentService(nil, nil, nil)
	_, err := service.CreateRechargeOrder(context.Background(), uuid.New(), "invalid_pack_id")
	if err == nil {
		t.Errorf("expected error for invalid pack id, got nil")
	}
}
