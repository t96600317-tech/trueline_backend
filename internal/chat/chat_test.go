package chat

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestChatService_NilPoolValidation(t *testing.T) {
	service := NewChatService(nil)
	ctx := context.Background()

	userID := uuid.New()
	listenerID := uuid.New()

	// 1. List Conversations with nil pool
	_, err := service.ListConversations(ctx, userID, "user")
	if err == nil {
		t.Errorf("Expected error when pool is nil, got nil")
	}

	// 2. Fetch Messages with nil pool
	_, err = service.GetChatMessages(ctx, userID, listenerID, "user")
	if err == nil {
		t.Errorf("Expected error when pool is nil, got nil")
	}

	// 3. Send Message with nil pool
	_, err = service.SendMessage(ctx, userID, listenerID, "user", "Hello")
	if err == nil {
		t.Errorf("Expected error when pool is nil, got nil")
	}
}
