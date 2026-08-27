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

	// Inspect all columns of wallets
	cRows, _ := pool.Query(ctx, `SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'wallets';`)
	fmt.Println("--- WALLETS COLUMNS ---")
	for cRows.Next() {
		var col, typ string
		_ = cRows.Scan(&col, &typ)
		fmt.Printf("Column: %s (%s)\n", col, typ)
	}
	cRows.Close()

	// 1. Inspect all wallets
	rows, err := pool.Query(ctx, `
		SELECT w.id, w.user_id, w.balance_micros, u.name
		FROM wallets w
		LEFT JOIN users u ON u.id = w.user_id;
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n--- WALLETS IN DB ---")
	for rows.Next() {
		var id, uid string
		var balance int64
		var name *string
		_ = rows.Scan(&id, &uid, &balance, &name)
		n := "NULL"
		if name != nil {
			n = *name
		}
		fmt.Printf("Wallet %s | User: %s (%s) | Balance: %d micros (₹%.2f)\n", id, n, uid, balance, float64(balance)/1000000.0)
	}
}
