package payments

import (
	"encoding/json"
	"io"
	"net/http"
	"trueline-backend/internal/auth"
)

type PaymentHandler struct {
	service  *PaymentService
	cfClient *CashfreePGClient
}

func NewPaymentHandler(service *PaymentService, cfClient *CashfreePGClient) *PaymentHandler {
	return &PaymentHandler{
		service:  service,
		cfClient: cfClient,
	}
}

type RechargeRequestPayload struct {
	PackID string `json:"pack_id"` // e.g. "pack_49", "pack_99", "pack_199"
}

func (h *PaymentHandler) GetCatalogue(w http.ResponseWriter, r *http.Request) {
	packs := make([]RechargePack, 0, len(RechargeCatalogue))
	for _, id := range []string{"pack_49", "pack_99", "pack_199"} {
		if p, ok := RechargeCatalogue[id]; ok {
			packs = append(packs, p)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    packs,
	})
}

func (h *PaymentHandler) InitiateRecharge(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "user" {
		http.Error(w, "Unauthorized: user role required", http.StatusUnauthorized)
		return
	}

	var req RechargeRequestPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request JSON", http.StatusBadRequest)
		return
	}

	if req.PackID == "" {
		http.Error(w, "pack_id is required (valid options: pack_49, pack_99, pack_199)", http.StatusBadRequest)
		return
	}

	result, err := h.service.CreateRechargeOrder(r.Context(), claims.UserID, req.PackID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    result,
	})
}

func (h *PaymentHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "user" {
		http.Error(w, "Unauthorized: user role required", http.StatusUnauthorized)
		return
	}

	var req struct {
		AmountPaise int64 `json:"amount_paise"`
		Coins       int64 `json:"coins"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request JSON", http.StatusBadRequest)
		return
	}

	packID := "pack_49"
	switch {
	case req.AmountPaise >= 19900:
		packID = "pack_199"
	case req.AmountPaise >= 9900:
		packID = "pack_99"
	default:
		packID = "pack_49"
	}

	result, err := h.service.CreateRechargeOrder(r.Context(), claims.UserID, packID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"order_id":           result.OrderID,
			"payment_session_id": result.PaymentSessionID,
			"order_status":       "ACTIVE",
		},
	})
}

func (h *PaymentHandler) VerifyOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil || claims.Role != "user" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	orderID := r.PathValue("id")
	if orderID == "" {
		http.Error(w, "order id required", http.StatusBadRequest)
		return
	}

	err := h.service.SettlePayment(r.Context(), orderID, "manual_verify", true)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": map[string]string{"status": "already_settled"}})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": map[string]string{"status": "settled"}})
}

func (h *PaymentHandler) CashfreeWebhook(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("x-webhook-signature")
	timestamp := r.Header.Get("x-webhook-timestamp")

	if h.cfClient != nil {
		if err := h.cfClient.VerifyWebhookSignature(signature, timestamp, bodyBytes); err != nil {
			http.Error(w, "Invalid webhook signature", http.StatusUnauthorized)
			return
		}
	}

	var event struct {
		Data struct {
			Order struct {
				OrderID string `json:"order_id"`
			} `json:"order"`
			Payment struct {
				CfPaymentID   string `json:"cf_payment_id"`
				PaymentStatus string `json:"payment_status"`
			} `json:"payment"`
		} `json:"data"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(bodyBytes, &event); err != nil {
		http.Error(w, "Invalid webhook body JSON", http.StatusBadRequest)
		return
	}

	orderID := event.Data.Order.OrderID
	paymentID := event.Data.Payment.CfPaymentID
	status := event.Data.Payment.PaymentStatus

	if event.Type == "PAYMENT_SUCCESS_WEBHOOK" || status == "SUCCESS" {
		err := h.service.SettlePayment(r.Context(), orderID, paymentID, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else if event.Type == "PAYMENT_FAILED_WEBHOOK" || status == "FAILED" {
		_ = h.service.SettlePayment(r.Context(), orderID, paymentID, false)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"OK"}`))
}
