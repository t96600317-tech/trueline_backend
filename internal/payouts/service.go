package payouts

import (
	"context"
	"errors"
	"fmt"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PayoutService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	cfClient *CashfreePayoutsClient
}

func NewPayoutService(pool *pgxpool.Pool, cfClient *CashfreePayoutsClient) *PayoutService {
	return &PayoutService{
		pool:     pool,
		queries:  db.New(pool),
		cfClient: cfClient,
	}
}

func (s *PayoutService) RequestPayout(ctx context.Context, listenerID uuid.UUID, amountMicros int64, upiID string) (*db.PayoutRequestGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	if amountMicros <= 0 {
		return nil, errors.New("payout amount must be positive")
	}

	availBalance, err := s.GetAvailableBalance(ctx, listenerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check available balance: %w", err)
	}

	if availBalance < amountMicros {
		return nil, fmt.Errorf("insufficient available earnings balance: %d micros available, %d requested", availBalance, amountMicros)
	}

	// 10% TDS default for pilot
	tdsMicros := amountMicros / 10
	netAmountMicros := amountMicros - tdsMicros

	req, err := s.queries.CreatePayoutRequest(ctx, db.CreatePayoutRequestParams{
		ListenerID:      pgtype.UUID{Bytes: listenerID, Valid: true},
		AmountMicros:    amountMicros,
		TdsMicros:       tdsMicros,
		NetAmountMicros: netAmountMicros,
		UpiID:           upiID,
	})
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func (s *PayoutService) GetAvailableBalance(ctx context.Context, listenerID uuid.UUID) (int64, error) {
	if s.pool == nil {
		return 0, errors.New("database not connected")
	}

	var currentBalance int64 = 0
	latestQuery := `SELECT balance_after_micros FROM earnings_ledger WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 1`
	err := s.pool.QueryRow(ctx, latestQuery, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&currentBalance)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}

	// Subtract pending payouts
	var pendingAmount int64 = 0
	pendingQuery := `SELECT COALESCE(SUM(amount_micros), 0)::BIGINT FROM payout_requests WHERE listener_id = $1 AND status = 'pending'`
	_ = s.pool.QueryRow(ctx, pendingQuery, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&pendingAmount)

	available := currentBalance - pendingAmount
	if available < 0 {
		return 0, nil
	}
	return available, nil
}

func (s *PayoutService) GetEarningsSummary(ctx context.Context, listenerID uuid.UUID) (totalEarned, totalPaid, currentBalance int64, err error) {
	if s.pool == nil {
		return 0, 0, 0, errors.New("database not connected")
	}

	summaryQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'call_credit' THEN amount_micros ELSE 0 END), 0)::BIGINT as earned,
			COALESCE(SUM(CASE WHEN type = 'payout' THEN amount_micros ELSE 0 END), 0)::BIGINT as paid
		FROM earnings_ledger
		WHERE listener_id = $1
	`
	err = s.pool.QueryRow(ctx, summaryQuery, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&totalEarned, &totalPaid)
	if err != nil {
		return 0, 0, 0, err
	}

	currentBalance, _ = s.GetAvailableBalance(ctx, listenerID)
	return totalEarned, totalPaid, currentBalance, nil
}

func (s *PayoutService) ProcessPayout(ctx context.Context, requestID, adminID uuid.UUID, approve bool) error {
	if s.pool == nil {
		return errors.New("database not connected")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var req db.PayoutRequestGenerated
	findQuery := `SELECT id, listener_id, amount_micros, tds_micros, net_amount_micros, status, upi_id FROM payout_requests WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(ctx, findQuery, pgtype.UUID{Bytes: requestID, Valid: true}).Scan(
		&req.ID, &req.ListenerID, &req.AmountMicros, &req.TdsMicros, &req.NetAmountMicros, &req.Status, &req.UpiID,
	)
	if err != nil {
		return fmt.Errorf("payout request not found: %w", err)
	}

	if req.Status != "pending" {
		return fmt.Errorf("payout request is already %s", req.Status)
	}

	if !approve {
		_, err = tx.Exec(ctx, `UPDATE payout_requests SET status = 'rejected', processed_at = NOW(), processed_by = $1 WHERE id = $2`,
			pgtype.UUID{Bytes: adminID, Valid: true}, req.ID)
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	// Transfer amount in INR
	netINR := float64(req.NetAmountMicros) / 1_000_000.0

	var transferRef = fmt.Sprintf("trf_%s", req.ID.String()[:8])
	if s.cfClient != nil {
		listenerUUID := uuid.UUID(req.ListenerID.Bytes)
		cfReq := DirectTransferRequest{
			TransferID:      transferRef,
			TransferAmount:  netINR,
			TransferMode:    "upi",
			TransferRemarks: "TrueLine Partner Payout",
		}
		cfReq.BeneficiaryDetails.BeneficiaryID = fmt.Sprintf("ben_%s", listenerUUID.String()[:8])
		cfReq.BeneficiaryDetails.BeneficiaryVPA = req.UpiID

		cfResp, err := s.cfClient.InitiateTransfer(ctx, cfReq)
		if err != nil {
			return fmt.Errorf("cashfree transfer failed: %w", err)
		}
		if cfResp.UTRExternal != "" {
			transferRef = cfResp.UTRExternal
		}
	}

	// 1. Mark request as paid
	_, err = tx.Exec(ctx, `UPDATE payout_requests SET status = 'paid', upi_ref = $1, processed_at = NOW(), processed_by = $2 WHERE id = $3`,
		pgtype.Text{String: transferRef, Valid: true}, pgtype.UUID{Bytes: adminID, Valid: true}, req.ID)
	if err != nil {
		return err
	}

	// 2. Fetch current balance to compute balance_after
	var lastBalance int64 = 0
	_ = tx.QueryRow(ctx, `SELECT balance_after_micros FROM earnings_ledger WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 1`, req.ListenerID).Scan(&lastBalance)
	newBalance := lastBalance - req.AmountMicros

	// 3. Write debit to earnings_ledger
	idempotencyKey := fmt.Sprintf("payout_%s", req.ID.String())
	_, err = tx.Exec(ctx, `
		INSERT INTO earnings_ledger (listener_id, type, amount_micros, balance_after_micros, reference_id, idempotency_key, tax_info)
		VALUES ($1, 'payout', $2, $3, $4, $5, $6)
	`, req.ListenerID, req.AmountMicros, newBalance, transferRef, idempotencyKey, []byte(fmt.Sprintf(`{"tds_micros": %d}`, req.TdsMicros)))
	if err != nil {
		return fmt.Errorf("failed to record earnings ledger payout: %w", err)
	}

	return tx.Commit(ctx)
}
