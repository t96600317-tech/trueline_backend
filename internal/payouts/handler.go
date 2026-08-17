package payouts

import (
	"encoding/json"
	"net/http"

	"trueline-backend/internal/auth"
)

type PayoutHandler struct {
	service *PayoutService
}

func NewPayoutHandler(service *PayoutService) *PayoutHandler {
	return &PayoutHandler{service: service}
}

type PayoutRequestPayload struct {
	AmountMicros int64  `json:"amount_micros"`
	UPIID        string `json:"upi_id"`
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

func (h *PayoutHandler) RequestPayout(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req PayoutRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	res, err := h.service.RequestPayout(r.Context(), claims.UserID, req.AmountMicros, req.UPIID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "PAYOUT_REQUEST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, res)
}

func (h *PayoutHandler) GetEarnings(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	earned, paid, balance, err := h.service.GetEarningsSummary(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "FETCH_EARNINGS_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_earned_micros":      earned,
		"total_earned_coins":       float64(earned) / 1_000_000.0,
		"total_paid_micros":        paid,
		"total_paid_coins":         float64(paid) / 1_000_000.0,
		"available_balance_micros": balance,
		"available_balance_coins":  float64(balance) / 1_000_000.0,
		"rate_per_minute_coins":    3.0,
	})
}
