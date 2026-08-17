package calls

import (
	"context"
	"fmt"
	"log"
	"time"

	"trueline-backend/internal/db"
	"trueline-backend/internal/earnings"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeteringEngine struct {
	pool            *pgxpool.Pool
	queries         *db.Queries
	walletService   *wallet.WalletService
	earningsService *earnings.EarningsService
	callService     *CallService
	hub             *EventHub
}

func NewMeteringEngine(pool *pgxpool.Pool, ws *wallet.WalletService, es *earnings.EarningsService, cs *CallService, hub *EventHub) *MeteringEngine {
	return &MeteringEngine{
		pool:            pool,
		queries:         db.New(pool),
		walletService:   ws,
		earningsService: es,
		callService:     cs,
		hub:             hub,
	}
}

func (e *MeteringEngine) Start(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	log.Println("Billing Metering Engine started...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.processTicks(ctx)
		}
	}
}

func (e *MeteringEngine) processTicks(ctx context.Context) {
	query := `SELECT id, user_id, listener_id, rate_per_min_micros_snapshot, earning_per_min_micros_snapshot FROM call_sessions WHERE status = 'active'`
	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var sessionID, userID, listenerID pgtype.UUID
		var rateMin, earningMin int64
		if err := rows.Scan(&sessionID, &userID, &listenerID, &rateMin, &earningMin); err != nil {
			continue
		}

		rateSec := rateMin / 60
		earningSec := earningMin / 60

		go e.billSecond(ctx, sessionID.Bytes, userID.Bytes, listenerID.Bytes, rateSec, earningSec)
	}
}

func (e *MeteringEngine) billSecond(ctx context.Context, sessionID, userID, listenerID uuid.UUID, rateSec, earningSec int64) {
	idempotencyKey := fmt.Sprintf("bill_%s_%d", sessionID.String(), time.Now().Unix())

	err := e.walletService.DebitWallet(ctx, userID, rateSec, "call_debit", sessionID.String(), idempotencyKey)
	if err != nil {
		if err.Error() == "insufficient balance" {
			log.Printf("Metering: Ending call %s due to zero balance", sessionID)
			_ = e.callService.EndCall(ctx, sessionID, uuid.Nil, "system", "low_balance")
			e.hub.Broadcast(sessionID, map[string]string{"type": "call_ended", "reason": "low_balance"})
		}
		return
	}

	_ = e.earningsService.CreditEarnings(ctx, listenerID, earningSec, sessionID.String(), idempotencyKey, nil)

	// Broadcast balance update (optional: every 5-10 seconds to reduce noise)
	e.hub.Broadcast(sessionID, map[string]interface{}{
		"type": "balance_updated",
		"session_id": sessionID,
	})
}
