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

	fmt.Println("Connected to Supabase PostgreSQL!")

	// 1. Add name column to users table if not exists
	alterQuery := `ALTER TABLE users ADD COLUMN IF NOT EXISTS name VARCHAR(100) DEFAULT '';`
	_, err = pool.Exec(ctx, alterQuery)
	if err != nil {
		log.Fatalf("Failed to add name column: %v", err)
	}
	fmt.Println("Ensured 'name' column exists in users table.")

	// 2. Populate default usernames for users where name is empty
	updateQuery := `
		UPDATE users 
		SET name = 'user' || RIGHT(REPLACE(id::text, '-', ''), 6)
		WHERE name IS NULL OR name = '';
	`
	tag, err := pool.Exec(ctx, updateQuery)
	if err != nil {
		log.Fatalf("Failed to populate default usernames: %v", err)
	}
	fmt.Printf("Updated %d users with default formatted usernames.\n", tag.RowsAffected())

	// 3. Inspect existing chat messages
	inspectChatQuery := `
		SELECT cm.id, cm.user_id, cm.listener_id, cm.sender_type, cm.content, u.name as user_name
		FROM chat_messages cm
		JOIN users u ON u.id = cm.user_id
		ORDER BY cm.created_at DESC
		LIMIT 10;
	`
	rows, err := pool.Query(ctx, inspectChatQuery)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		defer rows.Close()
		fmt.Println("\n--- RECENT CHAT MESSAGES ---")
		for rows.Next() {
			var id, uid, lid, sender, content, uName string
			_ = rows.Scan(&id, &uid, &lid, &sender, &content, &uName)
			fmt.Printf("Msg %s | Sender: %s | User: %s (%s) -> Listener: %s | Text: %s\n", id, sender, uName, uid, lid, content)
		}
	}
}
