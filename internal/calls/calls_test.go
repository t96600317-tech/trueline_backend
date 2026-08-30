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

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestZegoUserIDMatchesAndroidNormalization(t *testing.T) {
	userID := uuid.MustParse("fd39e827-1e24-4828-b8ed-08dc5a90a2b0")

	if got, want := zegoUserID(userID), "fd39e827_1e24_4828_b8ed_08dc5a90a2b0"; got != want {
		t.Fatalf("Zego user ID mismatch: got %q, want %q", got, want)
	}
}

func TestCallDurationSecondsUsesServerTimestamps(t *testing.T) {
	startedAt := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	endedAt := startedAt.Add(95 * time.Second)
	session := &db.CallSessionGenerated{
		StartedAt: pgtype.Timestamptz{Time: startedAt, Valid: true},
		EndedAt:   pgtype.Timestamptz{Time: endedAt, Valid: true},
	}

	if got, want := callDurationSeconds(session), int64(95); got != want {
		t.Fatalf("call duration mismatch: got %d, want %d", got, want)
	}
}

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
	nonceLength := int(binary.BigEndian.Uint16(encoded[8:10]))
	if nonceLength != 12 || len(encoded) < 10+nonceLength+2+1 {
		t.Fatalf("unexpected Token04 nonce envelope")
	}
	nonceStart := 10
	cipherLengthStart := nonceStart + nonceLength
	cipherLength := int(binary.BigEndian.Uint16(encoded[cipherLengthStart : cipherLengthStart+2]))
	cipherStart := cipherLengthStart + 2
	if cipherLength == 0 || len(encoded) != cipherStart+cipherLength+1 {
		t.Fatalf("unexpected token ciphertext envelope")
	}
	if mode := encoded[len(encoded)-1]; mode != 1 {
		t.Fatalf("expected AES-GCM Token04 mode byte 1, got %d", mode)
	}

	block, err := aes.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("failed to create test cipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("failed to create test GCM: %v", err)
	}
	plain, err := gcm.Open(nil, encoded[nonceStart:cipherLengthStart], encoded[cipherStart:cipherStart+cipherLength], nil)
	if err != nil {
		t.Fatalf("failed to decrypt Token04 AES-GCM payload: %v", err)
	}

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
