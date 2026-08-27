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

	// Update all users that have hex chars to pure numeric 6 digits (except user276067)
	updateQuery := `
		UPDATE users 
		SET name = 'user' || (100000 + (abs(hashtext(id::text)) % 900000))::text
		WHERE id != '9b4be774-bcd0-401b-95cf-96b083a500ec' AND (name IS NULL OR name ~* '[a-f]');
	`
	tag, err := pool.Exec(ctx, updateQuery)
	if err != nil {
		log.Fatalf("Update failed: %v", err)
	}
	fmt.Printf("Updated %d users with pure 6-digit numeric usernames.\n", tag.RowsAffected())

	// Ensure 9b4be774-bcd0-401b-95cf-96b083a500ec is definitely user276067
	_, _ = pool.Exec(ctx, `UPDATE users SET name = 'user276067' WHERE id = '9b4be774-bcd0-401b-95cf-96b083a500ec';`)

	rows, _ := pool.Query(ctx, `SELECT id, name FROM users;`)
	defer rows.Close()
	fmt.Println("\n--- FINAL USERS IN DB ---")
	for rows.Next() {
		var id, name string
		_ = rows.Scan(&id, &name)
		fmt.Printf("User %s -> %s\n", id, name)
	}
}
