package kyc

import (
	"encoding/json"
	"net/http"

	"trueline-backend/internal/auth"
)

type KYCHandler struct {
	service *KYCService
}

func NewKYCHandler(service *KYCService) *KYCHandler {
	return &KYCHandler{service: service}
}

type SubmitPANPayload struct {
	PAN string `json:"pan"`
}

type SubmitBankPayload struct {
	AccountNumber string `json:"account_number"`
	IFSC          string `json:"ifsc"`
}

type SubmitSelfiePayload struct {
	SelfieURL     string  `json:"selfie_url"`
	LivenessScore float64 `json:"liveness_score"`
}

type SubmitAgreementPayload struct {
	AgreementVersion string `json:"agreement_version"`
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

func (h *KYCHandler) SubmitPAN(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req SubmitPANPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	res, err := h.service.SubmitPAN(r.Context(), claims.UserID, req.PAN)
	if err != nil {
		writeError(w, http.StatusBadRequest, "PAN_VERIFICATION_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *KYCHandler) SubmitBank(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req SubmitBankPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	res, err := h.service.SubmitBank(r.Context(), claims.UserID, req.AccountNumber, req.IFSC)
	if err != nil {
		writeError(w, http.StatusBadRequest, "BANK_VERIFICATION_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *KYCHandler) SubmitSelfie(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req SubmitSelfiePayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	res, err := h.service.SubmitSelfieLiveness(r.Context(), claims.UserID, req.SelfieURL, req.LivenessScore)
	if err != nil {
		writeError(w, http.StatusBadRequest, "SELFIE_SUBMIT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func (h *KYCHandler) SubmitAgreement(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
		return
	}

	var req SubmitAgreementPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	res, err := h.service.SubmitAgreement(r.Context(), claims.UserID, req.AgreementVersion)
	if err != nil {
		writeError(w, http.StatusBadRequest, "AGREEMENT_SUBMIT_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}
