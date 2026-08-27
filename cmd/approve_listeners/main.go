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

	fmt.Println("Connected to Supabase PostgreSQL Database Pool!")

	// 1. Inspect all listeners
	rows, err := pool.Query(ctx, `SELECT id, name, onboarding_step, kyc_status, status, availability FROM listeners;`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n--- CURRENT LISTENERS IN DB ---")
	for rows.Next() {
		var id, name, step, kyc, status, avail string
		_ = rows.Scan(&id, &name, &step, &kyc, &status, &avail)
		fmt.Printf("Listener: %s (%s) | Step: %s | KYC: %s | Status: %s | Avail: %s\n", name, id, step, kyc, status, avail)
	}

	// 2. Approve ALL listeners so they bypass review & go live immediately
	approveQuery := `
		UPDATE listeners
		SET kyc_status = 'approved',
		    onboarding_step = 'approved',
		    status = 'active',
		    availability = 'online',
		    updated_at = NOW();
	`
	tag, err := pool.Exec(ctx, approveQuery)
	if err != nil {
		log.Fatalf("Approval update failed: %v", err)
	}
	fmt.Printf("\nSuccessfully approved %d listener(s) in database! All listeners are now Approved & Live.\n", tag.RowsAffected())
}
