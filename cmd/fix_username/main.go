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

	// Update user 9b4be774-bcd0-401b-95cf-96b083a500ec to user276067
	_, err = pool.Exec(ctx, `
		INSERT INTO users (id, name, phone_hash, encrypted_phone, language_pref, status)
		VALUES ('9b4be774-bcd0-401b-95cf-96b083a500ec', 'user276067', 'legacy_hash', 'legacy_phone', 'en', 'active')
		ON CONFLICT (id) DO UPDATE SET name = 'user276067', updated_at = NOW();
	`)
	if err != nil {
		log.Printf("Insert/update error: %v", err)
	} else {
		fmt.Println("Successfully updated user 9b4be774-bcd0-401b-95cf-96b083a500ec name to 'user276067'!")
	}

	// Update any other users that might have hex names to proper userXXXXXX
	rows, _ := pool.Query(ctx, `SELECT id, name FROM users;`)
	defer rows.Close()
	fmt.Println("\n--- CURRENT USERS ---")
	for rows.Next() {
		var id, name string
		_ = rows.Scan(&id, &name)
		fmt.Printf("User %s -> %s\n", id, name)
	}
}
