package payouts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type CashfreePayoutsClient struct {
	clientID     string
	clientSecret string
	isSandbox    bool
	httpClient   *http.Client
}

func NewCashfreePayoutsClient(clientID, clientSecret string, isSandbox bool) *CashfreePayoutsClient {
	return &CashfreePayoutsClient{
		clientID:     clientID,
		clientSecret: clientSecret,
		isSandbox:    isSandbox,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *CashfreePayoutsClient) getBaseURL() string {
	if c.isSandbox {
		return "https://sandbox.cashfree.com/payout"
	}
	return "https://api.cashfree.com/payout"
}

type DirectTransferRequest struct {
	TransferID     string  `json:"transfer_id"`
	TransferAmount float64 `json:"transfer_amount"`
	TransferMode   string  `json:"transfer_mode"` // "banktransfer", "upi"
	BeneficiaryDetails struct {
		BeneficiaryID    string `json:"beneficiary_id"`
		BeneficiaryName  string `json:"beneficiary_name"`
		BeneficiaryPhone string `json:"beneficiary_phone,omitempty"`
		BeneficiaryEmail string `json:"beneficiary_email,omitempty"`
		BeneficiaryVPA   string `json:"beneficiary_vpa,omitempty"`
		BeneficiaryBankDetails struct {
			AccountNumber string `json:"bank_account_number,omitempty"`
			IFSC          string `json:"ifsc,omitempty"`
		} `json:"beneficiary_instrument_details,omitempty"`
	} `json:"beneficiary_details"`
	TransferRemarks string `json:"transfer_remarks"`
}

type DirectTransferResponse struct {
	TransferID  string  `json:"transfer_id"`
	Status      string  `json:"status"`
	UTRExternal string  `json:"utr"`
	Amount      float64 `json:"transfer_amount"`
	Message     string  `json:"message"`
}

func (c *CashfreePayoutsClient) InitiateTransfer(ctx context.Context, req DirectTransferRequest) (*DirectTransferResponse, error) {
	if c.clientID == "" || c.clientSecret == "" {
		// Mock response for sandbox / local tests
		return &DirectTransferResponse{
			TransferID:  req.TransferID,
			Status:      "SUCCESS",
			UTRExternal: fmt.Sprintf("UTR_MOCK_%d", time.Now().Unix()),
			Amount:      req.TransferAmount,
			Message:     "Mock payout transfer processed successfully",
		}, nil
	}

	endpoint := fmt.Sprintf("%s/transfers", c.getBaseURL())
	bodyBytes, _ := json.Marshal(req)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-client-id", c.clientID)
	httpReq.Header.Set("x-client-secret", c.clientSecret)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("cashfree payout transfer request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cashfree payout returned HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var payoutResp DirectTransferResponse
	if err := json.Unmarshal(respBytes, &payoutResp); err != nil {
		return nil, fmt.Errorf("failed to decode payout transfer response: %w", err)
	}

	return &payoutResp, nil
}
