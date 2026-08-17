package earnings

import (
	"context"
	"encoding/json"
	"errors"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EarningsService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewEarningsService(pool *pgxpool.Pool) *EarningsService {
	return &EarningsService{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (s *EarningsService) CreditEarnings(ctx context.Context, listenerID uuid.UUID, amountMicros int64, referenceID, idempotencyKey string, taxInfo map[string]interface{}) error {
	if amountMicros <= 0 {
		return errors.New("earnings amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	_, err = qtx.GetEarningsLedgerByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// Get latest balance from ledger
	var currentBalance int64 = 0
	latestQuery := `SELECT balance_after_micros FROM earnings_ledger WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 1`
	err = tx.QueryRow(ctx, latestQuery, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&currentBalance)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	newBalance := currentBalance + amountMicros

	taxJSON, _ := json.Marshal(taxInfo)

	_, err = qtx.InsertEarningsLedgerEntry(ctx, db.InsertEarningsLedgerEntryParams{
		ListenerID:         pgtype.UUID{Bytes: listenerID, Valid: true},
		Type:               "call_credit",
		AmountMicros:       amountMicros,
		BalanceAfterMicros: newBalance,
		ReferenceID:        referenceID,
		IdempotencyKey:     idempotencyKey,
		TaxInfo:            taxJSON,
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
