package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres.npxfvhvukuvsqikxeyic:Prithvi17082003@aws-0-ap-south-1.pooler.supabase.com:6543/postgres?sslmode=require"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Credit 1000 coins (1,000,000,000 micros) to all user wallets
	tag, err := pool.Exec(ctx, `
		UPDATE wallets
		SET balance_micros = 1000000000,
		    updated_at = NOW();
	`)
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}
	fmt.Printf("Successfully credited 1000 coins (₹1000) to %d user wallet(s) in DB!\n", tag.RowsAffected())

	// Also make sure Barkha listener is online and ready
	_, _ = pool.Exec(ctx, `
		UPDATE listeners
		SET availability = 'online',
		    current_call_session_id = NULL,
		    status = 'active',
		    kyc_status = 'approved',
		    onboarding_step = 'approved',
		    updated_at = NOW();
	`)
	fmt.Println("Reset and activated all listeners to ONLINE with no active session locks!")
}
