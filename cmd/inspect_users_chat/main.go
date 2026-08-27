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

	// 1. Inspect all users
	rows, err := pool.Query(ctx, `SELECT id, name, language_pref, status, created_at FROM users ORDER BY created_at DESC LIMIT 20;`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n--- ALL USERS IN DB ---")
	for rows.Next() {
		var id, name, lang, status, created string
		_ = rows.Scan(&id, &name, &lang, &status, &created)
		fmt.Printf("User: ID=%s | Name=%s | Lang=%s | Status=%s | Created=%s\n", id, name, lang, status, created)
	}

	// 2. Inspect all chat messages
	msgRows, err := pool.Query(ctx, `
		SELECT cm.id, cm.user_id, cm.listener_id, cm.sender_type, cm.content, cm.created_at, u.name as user_name, l.name as listener_name
		FROM chat_messages cm
		LEFT JOIN users u ON u.id = cm.user_id
		LEFT JOIN listeners l ON l.id = cm.listener_id
		ORDER BY cm.created_at DESC
		LIMIT 10;
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer msgRows.Close()

	fmt.Println("\n--- RECENT CHAT MESSAGES IN DB ---")
	for msgRows.Next() {
		var id, uid, lid, sender, content, created string
		var uName, lName *string
		_ = msgRows.Scan(&id, &uid, &lid, &sender, &content, &created, &uName, &lName)
		uN := "NULL"
		if uName != nil {
			uN = *uName
		}
		lN := "NULL"
		if lName != nil {
			lN = *lName
		}
		fmt.Printf("Msg: %s | User: %s (%s) -> Listener: %s (%s) | Sender: %s | Content: %s | Created: %s\n",
			id, uN, uid, lN, lid, sender, content, created)
	}
}
