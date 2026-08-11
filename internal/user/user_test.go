package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserService_DiscoverPartners(t *testing.T) {
	service := NewUserService(nil)
	ctx := context.Background()
	userID := uuid.New()

	partners, err := service.ListDiscoverPartners(ctx, userID, "", "")
	if err != nil {
		t.Fatalf("Failed to list discover partners: %v", err)
	}

	if len(partners) == 0 {
		t.Errorf("Expected partners list to be non-empty")
	}

	// Test Language Filter
	hiPartners, err := service.ListDiscoverPartners(ctx, userID, "Hindi", "")
	if err != nil {
		t.Fatalf("Failed to list hindi discover partners: %v", err)
	}

	if len(hiPartners) == 0 {
		t.Errorf("Expected hindi partners to be non-empty")
	}

	// Test Search Query
	searchPartners, err := service.ListDiscoverPartners(ctx, userID, "", "Afreen")
	if err != nil {
		t.Fatalf("Failed to search partner Afreen: %v", err)
	}

	if len(searchPartners) == 0 || searchPartners[0].Name != "Afreen" {
		t.Errorf("Expected search result to return Afreen")
	}
}
