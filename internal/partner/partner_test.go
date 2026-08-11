package partner

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestPartnerService_UpdateProfileAndAvailability(t *testing.T) {
	service := NewPartnerService(nil)
	ctx := context.Background()
	partnerID := uuid.New()

	// 1. Update Profile
	profile, err := service.UpdateProfile(ctx, partnerID, UpdateProfilePayload{
		Name:       "Ananya Sharma",
		PhotoURL:   "https://photo.jpg",
		Bio:        "Empathetic listener",
		Languages:  []string{"hi", "en"},
		RatePerMin: 9.00,
	})

	if err != nil {
		t.Fatalf("Failed to update profile: %v", err)
	}

	if profile.Name != "Ananya Sharma" {
		t.Errorf("Expected name 'Ananya Sharma', got '%s'", profile.Name)
	}

	// 2. Set Availability to online should fail if KYC is pending
	_, err = service.SetAvailability(ctx, partnerID, "online")
	if err == nil {
		t.Errorf("Expected error when going online with pending KYC, but got nil")
	}

	// 3. Set Availability to offline should succeed
	profileOffline, err := service.SetAvailability(ctx, partnerID, "offline")
	if err != nil {
		t.Fatalf("Failed to set availability to offline: %v", err)
	}

	if profileOffline.Availability != "offline" {
		t.Errorf("Expected availability 'offline', got '%s'", profileOffline.Availability)
	}
}
