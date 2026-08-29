package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"trueline-backend/internal/db"
)

func main() {
	_ = godotenv.Load(".env")
	ctx := context.Background()
	database, err := db.ConnectSupabaseDB(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	rows, err := database.Pool.Query(ctx, "SELECT id, user_id, listener_id, sender_type, content, created_at FROM chat_messages ORDER BY created_at ASC")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- ALL CHAT MESSAGES IN DB ---")
	count := 0
	for rows.Next() {
		count++
		var id, uid, lid, sender, content string
		var created time.Time
		_ = rows.Scan(&id, &uid, &lid, &sender, &content, &created)
		fmt.Printf("[%d] Sender: %-8s | Content: %-25s | Time: %v\n", count, sender, content, created)
	}
	fmt.Printf("Total: %d messages\n", count)
}
