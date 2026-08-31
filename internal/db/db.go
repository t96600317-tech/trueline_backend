package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"trueline-backend/internal/db/generated"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Re-export generated Querier and New
type Querier = db.Querier
type Queries = db.Queries

func New(d db.DBTX) *db.Queries {
	return db.New(d)
}

// Re-export Model types from generated
type UserGenerated = db.User
type ListenerGenerated = db.Listener
type WalletGenerated = db.Wallet
type EarningsLedgerGenerated = db.EarningsLedger
type WalletLedgerGenerated = db.WalletLedger
type CallSessionGenerated = db.CallSession
type ChatMessageGenerated = db.ChatMessage
type KycRequestGenerated = db.KycRequest
type PayoutRequestGenerated = db.PayoutRequest

type ListPendingKYCRow = db.ListPendingKYCRow

// Re-export Params types if needed
type (
	CreateOTPRequestParams             = db.CreateOTPRequestParams
	GetLatestOTPParams                 = db.GetLatestOTPParams
	CreateUserParams                   = db.CreateUserParams
	UpdateUserLanguageParams           = db.UpdateUserLanguageParams
	CreateListenerParams               = db.CreateListenerParams
	UpdateListenerOnboardingStepParams = db.UpdateListenerOnboardingStepParams
	UpdateListenerProfileParams        = db.UpdateListenerProfileParams
	UpdateListenerKYCStatusParams      = db.UpdateListenerKYCStatusParams
	UpdateListenerAvailabilityParams   = db.UpdateListenerAvailabilityParams
	SetListenerCurrentCallSessionParams = db.SetListenerCurrentCallSessionParams
	CreateWalletParams                 = db.CreateWalletParams
	UpdateWalletBalanceParams          = db.UpdateWalletBalanceParams
	InsertWalletLedgerEntryParams      = db.InsertWalletLedgerEntryParams
	CreateCallSessionParams            = db.CreateCallSessionParams
	UpdateCallSessionStatusParams      = db.UpdateCallSessionStatusParams
	EndCallSessionParams               = db.EndCallSessionParams
	InsertEarningsLedgerEntryParams    = db.InsertEarningsLedgerEntryParams
	CreateKYCRequestParams             = db.CreateKYCRequestParams
	UpdateKYCRequestStatusParams       = db.UpdateKYCRequestStatusParams
	CreatePayoutRequestParams          = db.CreatePayoutRequestParams
	ProcessPayoutRequestParams         = db.ProcessPayoutRequestParams
	AddRatingParams                    = db.AddRatingParams
)

type Database struct {
	Pool *pgxpool.Pool
}

func ConnectSupabaseDB(ctx context.Context, databaseURL string) (*Database, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	// Use simple query protocol (QueryExecModeExec) for PgBouncer / Supabase Transaction Mode compatibility
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	config.ConnConfig.StatementCacheCapacity = 0
	config.ConnConfig.DescriptionCacheCapacity = 0
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnIdleTime = 15 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		log.Printf("Warning: Database ping failed (will retry): %v", err)
	} else {
		log.Println("Successfully connected to Supabase PostgreSQL Database Pool!")
	}

	return &Database{Pool: pool}, nil
}

func (d *Database) Close() {
	if d.Pool != nil {
		d.Pool.Close()
	}
}

type DBTX = db.DBTX
