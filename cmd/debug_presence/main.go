package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"trueline-backend/internal/chat"
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
	pool := database.Pool

	fmt.Println("=== 1. SERVER CURRENT TIME & USERS UPDATED_AT ===")
	var now time.Time
	_ = pool.QueryRow(ctx, "SELECT NOW()").Scan(&now)
	fmt.Printf("Database NOW(): %v\n", now)

	rows, err := pool.Query(ctx, `
		SELECT id, name, updated_at, NOW() - updated_at as age 
		FROM users 
		ORDER BY updated_at DESC LIMIT 10
	`)
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		var id, name string
		var updatedAt time.Time
		var age string
		_ = rows.Scan(&id, &name, &updatedAt, &age)
		fmt.Printf("User ID: %s | Name: %s | UpdatedAt: %v | Age: %s\n", id, name, updatedAt, age)
	}
	rows.Close()

	fmt.Println("\n=== 2. LISTENERS ===")
	lrows, err := pool.Query(ctx, `SELECT id, name, availability, updated_at FROM listeners LIMIT 5`)
	if err != nil {
		log.Fatal(err)
	}
	for lrows.Next() {
		var id, name, avail string
		var updatedAt time.Time
		_ = lrows.Scan(&id, &name, &avail, &updatedAt)
		fmt.Printf("Listener ID: %s | Name: %s | Avail: %s | UpdatedAt: %v\n", id, name, avail, updatedAt)
	}
	lrows.Close()

	fmt.Println("\n=== 3. CHAT SERVICE ListConversations FOR ALL ACTIVE LISTENERS ===")
	chatSvc := chat.NewChatService(pool)
	lrows2, _ := pool.Query(ctx, `SELECT id, name FROM listeners WHERE status = 'active'`)
	for lrows2.Next() {
		var lid, lname string
		_ = lrows2.Scan(&lid, &lname)
		uid := uuid.MustParse(lid)
		convs, err := chatSvc.ListConversations(ctx, uid, "listener")
		if err != nil {
			fmt.Printf("Error for listener %s (%s): %v\n", lname, lid, err)
			continue
		}
		fmt.Printf("Listener %s (%s) has %d conversations:\n", lname, lid, len(convs))
		for _, c := range convs {
			fmt.Printf("  -> User: %s (%s) | Availability: '%s' | LastMsg: '%s' | IsRegular: %v\n",
				c.UserName, c.UserID, c.UserAvailability, c.LastMessage, c.IsRegular)
		}
	}
	lrows2.Close()
}
