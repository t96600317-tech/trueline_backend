package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
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
	pgConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("Parse config failed: %v", err)
	}
	pgConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	pgConfig.ConnConfig.StatementCacheCapacity = 0
	pgConfig.ConnConfig.DescriptionCacheCapacity = 0

	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	// 1. Update wallet_ledger constraint
	_, err = pool.Exec(ctx, `
		ALTER TABLE wallet_ledger DROP CONSTRAINT IF EXISTS wallet_ledger_type_check;
		ALTER TABLE wallet_ledger ADD CONSTRAINT wallet_ledger_type_check 
			CHECK (type IN ('recharge', 'call_debit', 'chat_debit', 'chat_message', 'refund', 'admin_adjustment'));
	`)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Constraint updated successfully on wallet_ledger!")
}
