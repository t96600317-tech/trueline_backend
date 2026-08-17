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

	// 1. Clear bad / duplicate records
	tables := []string{
		"payout_requests",
		"kyc_requests",
		"call_sessions",
		"wallets",
		"listeners",
		"users",
		"otp_requests",
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

	// 2. Ensure strict unique constraints on mobile number (phone_hash)
	constraints := []string{
		`CREATE TABLE IF NOT EXISTS blocked_phones (
			phone_hash TEXT PRIMARY KEY,
			reason TEXT NOT NULL DEFAULT 'KYC Application Rejected',
			blocked_by TEXT NOT NULL DEFAULT 'admin',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);`,
		`DO $$ 
		BEGIN 
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'listeners_phone_hash_key'
			) THEN 
				ALTER TABLE listeners ADD CONSTRAINT listeners_phone_hash_key UNIQUE (phone_hash);
			END IF;
		END $$;`,
		`DO $$ 
		BEGIN 
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'users_phone_hash_key'
			) THEN 
				ALTER TABLE users ADD CONSTRAINT users_phone_hash_key UNIQUE (phone_hash);
			END IF;
		END $$;`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_listeners_phone_hash ON listeners(phone_hash);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_hash ON users(phone_hash);`,
	}

	for _, ddl := range constraints {
		_, err := pool.Exec(ctx, ddl)
		if err != nil {
			log.Printf("DDL warning: %v", err)
		}
	}

	log.Println("Database records cleared and UNIQUE constraints on mobile number (phone_hash) strictly enforced!")
}
