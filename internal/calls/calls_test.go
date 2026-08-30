package calls

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestZegoTokenProvider_GenerateToken04(t *testing.T) {
	provider := NewZegoTokenProvider("123456789", "0123456789abcdef0123456789abcdef")

	token, err := provider.GenerateToken("user_test_1", "room_test_1", 1*time.Hour)
	if err != nil {
		t.Fatalf("failed to generate zego token: %v", err)
	}

	if !strings.HasPrefix(token, "04") {
		t.Errorf("expected Zego token to have '04' prefix, got %s", token)
	}

	if len(token) < 50 {
		t.Errorf("token length suspiciously short: %d", len(token))
	}

	encoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(token, "04"))
	if err != nil {
		t.Fatalf("token is not valid base64: %v", err)
	}
	if len(encoded) < 12 {
		t.Fatalf("token envelope is too short: %d", len(encoded))
	}

	expire := int64(binary.BigEndian.Uint64(encoded[:8]))
	ivLength := int(binary.BigEndian.Uint16(encoded[8:10]))
	if ivLength != aes.BlockSize || len(encoded) < 10+ivLength+2 {
		t.Fatalf("unexpected token IV envelope")
	}
	ivStart := 10
	cipherLengthStart := ivStart + ivLength
	cipherLength := int(binary.BigEndian.Uint16(encoded[cipherLengthStart : cipherLengthStart+2]))
	cipherStart := cipherLengthStart + 2
	if cipherLength == 0 || len(encoded) != cipherStart+cipherLength {
		t.Fatalf("unexpected token ciphertext envelope")
	}

	block, err := aes.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("failed to create test cipher: %v", err)
	}
	plain := make([]byte, cipherLength)
	cipher.NewCBCDecrypter(block, encoded[ivStart:cipherLengthStart]).CryptBlocks(plain, encoded[cipherStart:])
	plain = plain[:len(plain)-int(plain[len(plain)-1])]

	var payload zegoToken04Payload
	if err := json.Unmarshal(plain, &payload); err != nil {
		t.Fatalf("failed to decode Token04 payload: %v", err)
	}
	if payload.AppID != 123456789 || payload.UserID != "user_test_1" || payload.ExpireTime != expire {
		t.Fatalf("unexpected Token04 claims: %+v", payload)
	}
	if payload.Nonce < 0 {
		t.Fatalf("Token04 nonce must be a non-negative int32, got %d", payload.Nonce)
	}
}

func TestZegoTokenProviderRejectsInvalidCredentials(t *testing.T) {
	tests := []struct {
		name     string
		appID    string
		secret   string
		wantText string
	}{
		{name: "invalid app ID", appID: "not-a-number", secret: "0123456789abcdef0123456789abcdef", wantText: "app ID"},
		{name: "invalid secret", appID: "123456789", secret: "too-short", wantText: "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewZegoTokenProvider(tt.appID, tt.secret).GenerateToken("user", "room", time.Hour)
			if err == nil || !strings.Contains(err.Error(), tt.wantText) {
				t.Fatalf("expected error containing %q, got %v", tt.wantText, err)
			}
		})
	}
}
