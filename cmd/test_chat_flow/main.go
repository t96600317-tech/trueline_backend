package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"trueline-backend/internal/chat"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
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

	userID := uuid.MustParse("9b4be774-bcd0-401b-95cf-96b083a500ec")
	palakID := uuid.MustParse("54d0b566-5183-44a3-8a8a-d0e1b824d71f")

	ws := wallet.NewWalletService(pool)
	cs := chat.NewChatService(pool, ws, nil)

	// Test DebitWallet directly to see the real underlying error
	msgID := uuid.New()
	idempotencyKey := fmt.Sprintf("chat_msg_%s", msgID.String())
	err = ws.DebitWallet(ctx, userID, 300000, "chat_message", msgID.String(), idempotencyKey)
	if err != nil {
		fmt.Printf("ACTUAL DebitWallet ERROR: %v (type: %T)\n", err, err)
	} else {
		fmt.Println("DebitWallet SUCCEEDED directly!")
	}

	// 1. Send test message
	msg, err := cs.SendMessage(ctx, userID, palakID, "user", "Hello Palak from user test!")
	if err != nil {
		log.Fatalf("SendMessage failed: %v", err)
	}
	fmt.Printf("SendMessage SUCCESS: ID=%s Content=%s CreatedAt=%v\n", msg.ID, msg.Content, msg.CreatedAt)

	// 2. List conversations for Palak (role = listener)
	lConvs, err := cs.ListConversations(ctx, palakID, "listener")
	if err != nil {
		log.Fatalf("ListConversations for listener failed: %v", err)
	}
	fmt.Printf("\n--- PALAK (LISTENER) CONVERSATIONS (%d found) ---\n", len(lConvs))
	for _, c := range lConvs {
		fmt.Printf("Conv: PartnerID=%s PartnerName=%s UserID=%v UserName=%s LastMsg=%s Unread=%d\n",
			c.PartnerID, c.PartnerName, c.UserID, c.UserName, c.LastMessage, c.UnreadCount)
	}

	// 3. List conversations for User (role = user)
	uConvs, err := cs.ListConversations(ctx, userID, "user")
	if err != nil {
		log.Fatalf("ListConversations for user failed: %v", err)
	}
	fmt.Printf("\n--- USER CONVERSATIONS (%d found) ---\n", len(uConvs))
	for _, c := range uConvs {
		fmt.Printf("Conv: PartnerID=%s PartnerName=%s ListenerID=%s ListenerName=%s LastMsg=%s Unread=%d\n",
			c.PartnerID, c.PartnerName, c.ListenerID, c.ListenerName, c.LastMessage, c.UnreadCount)
	}
}
