package listeners

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"trueline-backend/internal/auth"
)

type ListenerHandler struct {
	service *ListenerService
}

func NewListenerHandler(service *ListenerService) *ListenerHandler {
	return &ListenerHandler{service: service}
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

func (h *ListenerHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	profile, err := h.service.GetListenerProfile(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusNotFound, "LISTENER_NOT_FOUND", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ListenerHandler) UpdateOnboardingProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req UpdateProfilePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Title = strings.TrimSpace(req.Title)
	if len(req.Name) < 2 {
		writeError(w, http.StatusBadRequest, "INVALID_NAME", "Full name must be at least 2 characters")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "INVALID_TITLE", "City and region information is required")
		return
	}
	if len(req.Languages) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_LANGUAGES", "At least one language must be selected")
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), claims.UserID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_PROFILE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ListenerHandler) UpdateVoiceIntro(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req struct {
		AudioURL string `json:"audio_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AudioURL == "" {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "audio_url is required")
		return
	}

	profile, err := h.service.UpdateVoiceIntro(r.Context(), claims.UserID, req.AudioURL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "UPDATE_VOICE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ListenerHandler) SubmitOnboarding(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	profile, err := h.service.SubmitOnboarding(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "SUBMIT_ONBOARDING_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, profile)
}

func (h *ListenerHandler) SetAvailability(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req struct {
		Availability string `json:"availability"` // "online" | "offline"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	err := h.service.SetAvailability(r.Context(), claims.UserID, req.Availability)
	if err != nil {
		writeError(w, http.StatusBadRequest, "SET_AVAILABILITY_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"availability": req.Availability,
	})
}

func (h *ListenerHandler) GetHomeDashboard(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetHomeDashboard(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_DASHBOARD_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetMilestones(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetMilestonesHub(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_MILESTONES_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetPerformanceScore(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetPerformanceScore(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_SCORE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetDetailedEarnings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetDetailedEarnings(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_EARNINGS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) RequestWithdrawal(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req WithdrawRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	data, err := h.service.RequestWithdrawal(r.Context(), claims.UserID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "WITHDRAWAL_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetBlockedUsers(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_BLOCKED_USERS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) SubmitReport(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req struct {
		Reason  string `json:"reason"`
		Details string `json:"details"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	err := h.service.SubmitReport(r.Context(), claims.UserID, req.Reason, req.Details)
	if err != nil {
		writeError(w, http.StatusBadRequest, "REPORT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Report submitted successfully. Our safety team will review within 24 hours.",
	})
}

func (h *ListenerHandler) GetCallHistory(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetCallHistory(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_CALL_HISTORY_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetTransactions(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetTransactions(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_TRANSACTIONS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	data, err := h.service.GetNotifications(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_NOTIFICATIONS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, data)
}

func (h *ListenerHandler) NotifyMe(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 4 {
		writeError(w, http.StatusBadRequest, "INVALID_PATH", "Listener ID missing")
		return
	}
	listenerIDStr := parts[3]
	listenerID, err := uuid.Parse(listenerIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_UUID", "Invalid listener ID")
		return
	}

	if err := h.service.SubscribeNotifyWhenOnline(r.Context(), claims.UserID, listenerID); err != nil {
		writeError(w, http.StatusInternalServerError, "SUBSCRIBE_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"subscribed": true,
		"message":    "You will be notified as soon as the listener is back online!",
	})
}




