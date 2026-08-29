package chat

import (
	"encoding/json"
	"net/http"

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

	if claims.Role == "user" {
		h.service.TouchUserPresence(r.Context(), claims.UserID)
	}

	conversations, err := h.service.ListConversations(r.Context(), claims.UserID, claims.Role)
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

	targetIDStr := r.PathValue("id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid target ID")
		return
	}

	var userID, listenerID uuid.UUID
	if claims.Role == "user" {
		userID = claims.UserID
		listenerID = targetID
		h.service.TouchUserPresence(r.Context(), claims.UserID)
	} else {
		userID = targetID
		listenerID = claims.UserID
	}

	messages, err := h.service.GetChatMessages(r.Context(), userID, listenerID, claims.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_MESSAGES_FAILED", err.Error())
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

	targetIDStr := r.PathValue("id")
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid target ID")
		return
	}

	var req SendMessagePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	var userID, listenerID uuid.UUID
	if claims.Role == "user" {
		userID = claims.UserID
		listenerID = targetID
		h.service.TouchUserPresence(r.Context(), claims.UserID)
	} else {
		userID = targetID
		listenerID = claims.UserID
	}

	msg, err := h.service.SendMessage(r.Context(), userID, listenerID, claims.Role, req.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, "SEND_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}
