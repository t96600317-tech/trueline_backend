package listeners

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestListenerService_SetAvailabilityNilPool(t *testing.T) {
	service := NewListenerService(nil)
	ctx := context.Background()
	listenerID := uuid.New()

	err := service.SetAvailability(ctx, listenerID, "invalid_status")
	if err == nil {
		t.Errorf("expected error for invalid availability status, got nil")
	}

	err = service.SetAvailability(ctx, listenerID, "online")
	if err == nil {
		t.Errorf("expected error when pool is nil, got nil")
	}
}
