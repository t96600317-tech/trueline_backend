package chat

import (
	"encoding/json"
	"net/http"
	"strconv"

	"trueline-backend/internal/auth"

	"github.com/google/uuid"
)

type ChatHandler struct {
	service *ChatService
}

func NewChatHandler(service *ChatService) *ChatHandler {
	return &ChatHandler{service: service}
}

type response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *errorBody  `json:"error,omitempty"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{Success: true, Data: data})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response{
		Success: false,
		Error:   &errorBody{Code: code, Message: message},
	})
}

func (h *ChatHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	conversations, err := h.service.ListUserConversations(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_LIST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, conversations)
}

func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	partnerIDStr := r.PathValue("partner_id")
	partnerID, err := uuid.Parse(partnerIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARTNER_ID", "Partner ID must be a valid UUID")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	messages, err := h.service.GetChatMessages(r.Context(), claims.UserID, partnerID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "CHAT_MESSAGES_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, messages)
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	partnerIDStr := r.PathValue("partner_id")
	partnerID, err := uuid.Parse(partnerIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARTNER_ID", "Partner ID must be a valid UUID")
		return
	}

	var req SendMessagePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	msg, err := h.service.SendMessage(r.Context(), claims.UserID, partnerID, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, "SEND_MESSAGE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, msg)
}

func (h *ChatHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	partnerIDStr := r.PathValue("partner_id")
	partnerID, err := uuid.Parse(partnerIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_PARTNER_ID", "Partner ID must be a valid UUID")
		return
	}

	if err := h.service.MarkMessagesRead(r.Context(), claims.UserID, partnerID); err != nil {
		writeError(w, http.StatusInternalServerError, "MARK_READ_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Messages marked as read"})
}
