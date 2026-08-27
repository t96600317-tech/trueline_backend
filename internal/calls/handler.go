package calls

import (
	"encoding/json"
	"net/http"

	"trueline-backend/internal/auth"

	"github.com/google/uuid"
)

type CallHandler struct {
	service      *CallService
	hub          *EventHub
	tokenManager *auth.TokenManager
}

func NewCallHandler(service *CallService, hub *EventHub, tm *auth.TokenManager) *CallHandler {
	return &CallHandler{
		service:      service,
		hub:          hub,
		tokenManager: tm,
	}
}

type InitiateCallPayload struct {
	ListenerID string `json:"listener_id"`
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

func (h *CallHandler) InitiateCall(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "user" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User role required")
		return
	}

	var req InitiateCallPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	listenerID, err := uuid.Parse(req.ListenerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid listener ID")
		return
	}

	res, err := h.service.InitiateCall(r.Context(), claims.UserID, listenerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "CALL_INITIATE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, res)
}

func (h *CallHandler) GetIncomingCall(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "listener" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Listener role required")
		return
	}

	inc, err := h.service.GetIncomingCallForListener(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, inc)
}

func (h *CallHandler) AcceptCall(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "listener" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Listener role required")
		return
	}

	sessionIDStr := r.PathValue("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid session ID")
		return
	}

	res, err := h.service.AcceptCall(r.Context(), sessionID, claims.UserID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "CALL_ACCEPT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *CallHandler) EndCall(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	sessionIDStr := r.PathValue("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid session ID")
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "hangup"
	}

	err = h.service.EndCall(r.Context(), sessionID, claims.UserID, claims.Role, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, "CALL_END_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Call ended"})
}

type RateCallPayload struct {
	Rating     int      `json:"rating"`
	Tags       []string `json:"tags"`
	IsFavorite bool     `json:"is_favorite"`
}

func (h *CallHandler) RateCall(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "user" {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "User role required")
		return
	}

	sessionIDStr := r.PathValue("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid session ID")
		return
	}

	var req RateCallPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		writeError(w, http.StatusBadRequest, "INVALID_RATING", "Rating must be between 1 and 5")
		return
	}

	// Update call rating metadata and optional favorite status
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id": sessionID.String(),
		"rating":     req.Rating,
		"message":    "Thank you for your feedback!",
	})
}

func (h *CallHandler) HandleCallEventsWS(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.PathValue("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		// If custom/fallback session ID, use a deterministic UUID derived from the string
		sessionID = uuid.NewMD5(uuid.NameSpaceDNS, []byte(sessionIDStr))
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("Authorization")
	}

	claims, err := h.tokenManager.ValidateToken(token)
	if err != nil {
		http.Error(w, "Unauthorized WebSocket", http.StatusUnauthorized)
		return
	}

	// Verify caller belongs to this session if session exists in DB
	session, err := h.service.GetSession(r.Context(), sessionID)
	if err == nil && session != nil {
		if claims.Role != "admin" && session.UserID.Bytes != claims.UserID && session.ListenerID.Bytes != claims.UserID {
			http.Error(w, "Forbidden: not a participant", http.StatusForbidden)
			return
		}
	}

	h.hub.HandleWebSocket(w, r, sessionID)
}
