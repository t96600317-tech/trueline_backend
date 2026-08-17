package calls

import (
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
}
