package wallet

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

type WalletService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewWalletService(pool *pgxpool.Pool) *WalletService {
	return &WalletService{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (s *WalletService) CreditWallet(ctx context.Context, userID uuid.UUID, amountMicros int64, type_ string, referenceID, idempotencyKey string) error {
	if amountMicros <= 0 {
		return errors.New("credit amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// 1. Check for existing transaction with this idempotency key
	_, err = qtx.GetWalletLedgerByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		// Already processed
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	// 2. Lock wallet and get current balance
	wallet, err := qtx.GetWalletByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return fmt.Errorf("failed to get wallet: %w", err)
	}

	newBalance := wallet.BalanceMicros + amountMicros

	// 3. Update balance
	_, err = qtx.UpdateWalletBalance(ctx, db.UpdateWalletBalanceParams{
		ID:            wallet.ID,
		BalanceMicros: newBalance,
	})
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	// 4. Insert ledger entry
	_, err = qtx.InsertWalletLedgerEntry(ctx, db.InsertWalletLedgerEntryParams{
		WalletID:           wallet.ID,
		Type:               type_,
		AmountMicros:       amountMicros,
		BalanceAfterMicros: newBalance,
		ReferenceID:        referenceID,
		IdempotencyKey:     idempotencyKey,
		Description:        fmt.Sprintf("Credit: %s", type_),
	})
	if err != nil {
		return fmt.Errorf("failed to insert ledger entry: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *WalletService) DebitWallet(ctx context.Context, userID uuid.UUID, amountMicros int64, type_ string, referenceID, idempotencyKey string) error {
	if amountMicros <= 0 {
		return errors.New("debit amount must be positive")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	_, err = qtx.GetWalletLedgerByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		return nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	wallet, err := qtx.GetWalletByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			w, cErr := qtx.CreateWallet(ctx, db.CreateWalletParams{
				UserID:        pgtype.UUID{Bytes: userID, Valid: true},
				BalanceMicros: 1000_000_000,
			})
			if cErr != nil {
				return fmt.Errorf("failed to create initial user wallet: %w", cErr)
			}
			wallet = w
		} else {
			return err
		}
	}

	if wallet.BalanceMicros < amountMicros {
		return errors.New("insufficient balance")
	}

	newBalance := wallet.BalanceMicros - amountMicros

	_, err = qtx.UpdateWalletBalance(ctx, db.UpdateWalletBalanceParams{
		ID:            wallet.ID,
		BalanceMicros: newBalance,
	})
	if err != nil {
		return err
	}

	_, err = qtx.InsertWalletLedgerEntry(ctx, db.InsertWalletLedgerEntryParams{
		WalletID:           wallet.ID,
		Type:               type_,
		AmountMicros:       -amountMicros,
		BalanceAfterMicros: newBalance,
		ReferenceID:        referenceID,
		IdempotencyKey:     idempotencyKey,
		Description:        fmt.Sprintf("Debit: %s", type_),
	})
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
