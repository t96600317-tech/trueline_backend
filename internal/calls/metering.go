package calls

import (
	"context"
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
	query := `SELECT id, rate_per_min_micros_snapshot, started_at FROM call_sessions WHERE status = 'active'`
	rows, err := e.pool.Query(ctx, query)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var sessionID pgtype.UUID
		var rateMin int64
		var startedAt pgtype.Timestamptz
		if err := rows.Scan(&sessionID, &rateMin, &startedAt); err != nil {
			continue
		}
		if !startedAt.Valid {
			continue
		}
		elapsedSeconds := int64(time.Since(startedAt.Time).Seconds())
		go e.reserveCurrentMinute(ctx, sessionID.Bytes, customerReservationMicros(rateMin, elapsedSeconds))
	}
}

func (e *MeteringEngine) reserveCurrentMinute(ctx context.Context, sessionID uuid.UUID, targetMicros int64) {
	err := e.callService.ReserveCustomerCharge(ctx, sessionID, targetMicros)
	if err != nil {
		if err.Error() == "insufficient balance" {
			log.Printf("Metering: Ending call %s due to insufficient balance for the next rounded minute", sessionID)
			_ = e.callService.EndCall(ctx, sessionID, uuid.Nil, "system", "low_balance")
		}
		return
	}

	// The customer wallet changes only when a new minute is reserved.
	e.hub.Broadcast(sessionID, map[string]interface{}{
		"type":       "balance_updated",
		"session_id": sessionID,
	})
}
