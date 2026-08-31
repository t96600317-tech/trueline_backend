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

	// 1. Inspect all listeners
	lRows, err := pool.Query(ctx, `SELECT id, name, title, status, phone_hash, created_at FROM listeners ORDER BY created_at DESC LIMIT 20;`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer lRows.Close()

	fmt.Println("\n--- ALL LISTENERS IN DB ---")
	for lRows.Next() {
		var id, name, title, status, phoneHash, created string
		_ = lRows.Scan(&id, &name, &title, &status, &phoneHash, &created)
		fmt.Printf("Listener: ID=%s | Name=%s | Title=%s | Status=%s\n", id, name, title, status)
	}

	// 2. Inspect all chat messages
	msgRows, err := pool.Query(ctx, `
		SELECT cm.id, cm.user_id, cm.listener_id, cm.sender_type, cm.content, cm.created_at, 
		       COALESCE(u.name, 'NULL_USER_NAME'), COALESCE(l.name, 'NULL_LISTENER_NAME')
		FROM chat_messages cm
		LEFT JOIN users u ON u.id = cm.user_id
		LEFT JOIN listeners l ON l.id = cm.listener_id
		ORDER BY cm.created_at DESC
		LIMIT 20;
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer msgRows.Close()

	fmt.Println("\n--- RECENT CHAT MESSAGES IN DB ---")
	for msgRows.Next() {
		var id, uid, lid, sender, content, created, uName, lName string
		_ = msgRows.Scan(&id, &uid, &lid, &sender, &content, &created, &uName, &lName)
		fmt.Printf("Msg: %s | User: %s (%s) -> Listener: %s (%s) | Sender: %s | Content: %s | Created: %s\n", id, uName, uid, lName, lid, sender, content, created)
	}
}
