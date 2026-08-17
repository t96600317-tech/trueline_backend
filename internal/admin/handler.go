package admin

import (
	"encoding/json"
	"net/http"

	"trueline-backend/internal/auth"

	"github.com/google/uuid"
)

type AdminHandler struct {
	service *AdminService
}

func NewAdminHandler(service *AdminService) *AdminHandler {
	return &AdminHandler{service: service}
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

func (h *AdminHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	token, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "LOGIN_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"token": token,
	})
}

func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "GET_STATS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *AdminHandler) ListKYCQueue(w http.ResponseWriter, r *http.Request) {
	queue, err := h.service.ListKYCQueue(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_KYC_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, queue)
}

type ReviewKYCPayload struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (h *AdminHandler) ReviewKYC(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Admin authentication required")
		return
	}

	kycIDStr := r.PathValue("id")
	kycID, err := uuid.Parse(kycIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid KYC ID")
		return
	}

	var req ReviewKYCPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	err = h.service.ReviewKYC(r.Context(), kycID, claims.UserID, req.Status, req.Reason)
	if err != nil {
		writeError(w, http.StatusBadRequest, "REVIEW_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "KYC reviewed successfully"})
}

func (h *AdminHandler) ListListeners(w http.ResponseWriter, r *http.Request) {
	listeners, err := h.service.ListListeners(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_LISTENERS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, listeners)
}

func (h *AdminHandler) ToggleListenerStatus(w http.ResponseWriter, r *http.Request) {
	listenerIDStr := r.PathValue("id")
	listenerID, err := uuid.Parse(listenerIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid Listener ID")
		return
	}

	var req struct {
		Status string `json:"status"` // "active" | "blocked"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	err = h.service.ToggleListenerStatus(r.Context(), listenerID, req.Status)
	if err != nil {
		writeError(w, http.StatusBadRequest, "TOGGLE_STATUS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Listener status updated"})
}

func (h *AdminHandler) ListPayouts(w http.ResponseWriter, r *http.Request) {
	payouts, err := h.service.ListPayoutRequests(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_PAYOUTS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, payouts)
}

func (h *AdminHandler) ProcessPayout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Admin authentication required")
		return
	}

	payoutIDStr := r.PathValue("id")
	payoutID, err := uuid.Parse(payoutIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_ID", "Invalid Payout ID")
		return
	}

	var req struct {
		Approve bool `json:"approve"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	err = h.service.ProcessPayout(r.Context(), payoutID, claims.UserID, req.Approve)
	if err != nil {
		writeError(w, http.StatusBadRequest, "PROCESS_PAYOUT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Payout processed successfully"})
}

func (h *AdminHandler) ListLedgers(w http.ResponseWriter, r *http.Request) {
	ledgers, err := h.service.ListLedgers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_LEDGERS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ledgers)
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_USERS_FAILED", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, users)
}
