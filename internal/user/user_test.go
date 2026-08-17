package user

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestUserService_NilPoolValidation(t *testing.T) {
	service := NewUserService(nil)
	ctx := context.Background()
	userID := uuid.New()

	_, _, err := service.GetUserProfile(ctx, userID)
	if err == nil {
		t.Errorf("Expected error when pool is nil, got nil")
	}

	_, err = service.ListDiscoverListeners(ctx, userID, "", "")
	if err == nil {
		t.Errorf("Expected error when pool is nil, got nil")
	}
}
