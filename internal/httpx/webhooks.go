package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"trueline-backend/internal/calls"
)

type WebhookHandler struct {
	callService *calls.CallService
}

func NewWebhookHandler(callService *calls.CallService) *WebhookHandler {
	return &WebhookHandler{callService: callService}
}

func (h *WebhookHandler) ZegoWebhook(w http.ResponseWriter, r *http.Request) {
	// TODO: Verify Zego signature

	var event struct {
		Action string `json:"action"`
		RoomID string `json:"room_id"`
		UserID string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		log.Printf("Webhook: Failed to parse Zego event: %v", err)
		return
	}

	log.Printf("Webhook: Zego event received: %s in room %s", event.Action, event.RoomID)

	// Action types: room_login, room_logout
	// In v1, we primarily rely on the explicit 'end' API, but webhooks provide
	// the fallback for dropped connections.

	w.WriteHeader(http.StatusOK)
}
