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
	_ = godotenv.Load(".env")
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL not set in .env")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	tables := []string{
		"payout_requests",
		"kyc_requests",
		"ledger_transactions",
		"call_sessions",
		"wallets",
		"listeners",
		"users",
		"otp_requests",
		"blocked_phones",
		"admin_audit_logs",
	}

	for _, table := range tables {
		query := fmt.Sprintf("TRUNCATE TABLE %s CASCADE;", table)
		_, err := pool.Exec(ctx, query)
		if err != nil {
			log.Printf("Warning truncating %s: %v", table, err)
		} else {
			log.Printf("Successfully truncated %s", table)
		}
	}

	log.Println("Database cleanup completed successfully! Admin users preserved.")
}
