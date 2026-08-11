package chat

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestChatService_ListAndSend(t *testing.T) {
	service := NewChatService(nil)
	ctx := context.Background()

	userID := uuid.New()
	partnerID := uuid.MustParse("a0000000-0000-0000-0000-000000000001")

	// 1. List Conversations
	convs, err := service.ListUserConversations(ctx, userID)
	if err != nil {
		t.Fatalf("Failed to list conversations: %v", err)
	}

	if len(convs) == 0 {
		t.Errorf("Expected conversations list to be non-empty")
	}

	// 2. Fetch Messages
	messages, err := service.GetChatMessages(ctx, userID, partnerID, 10, 0)
	if err != nil {
		t.Fatalf("Failed to get chat messages: %v", err)
	}

	if len(messages) == 0 {
		t.Errorf("Expected chat messages to be non-empty")
	}

	// 3. Send Message
	msgText := "Hello Afreen! Testing chat message."
	sentMsg, err := service.SendMessage(ctx, userID, partnerID, msgText)
	if err != nil {
		t.Fatalf("Failed to send chat message: %v", err)
	}

	if sentMsg.Content != msgText {
		t.Errorf("Expected sent message content '%s', got '%s'", msgText, sentMsg.Content)
	}
}
