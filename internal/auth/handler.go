package auth

import (
	"encoding/json"
	"net/http"
)

type AuthHandler struct {
	service *AuthService
}

func NewAuthHandler(service *AuthService) *AuthHandler {
	return &AuthHandler{service: service}
}

type OTPRequestPayload struct {
	Phone string `json:"phone"`
	Role  string `json:"role"` // "user" or "partner"
}

type OTPVerifyPayload struct {
	Phone string `json:"phone"`
	OTP   string `json:"otp"`
	Role  string `json:"role"` // "user" or "partner"
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

func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed")
		return
	}

	var req OTPRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	resp, err := h.service.RequestOTP(r.Context(), req.Phone, req.Role)
	if err != nil {
		writeError(w, http.StatusBadRequest, "OTP_REQUEST_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed")
		return
	}

	var req OTPVerifyPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", "Failed to parse JSON body")
		return
	}

	if req.Role == "" {
		req.Role = "user"
	}

	resp, err := h.service.VerifyOTP(r.Context(), req.Phone, req.OTP, req.Role)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "OTP_VERIFICATION_FAILED", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
