package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"trueline-backend/internal/calls"
	"trueline-backend/internal/db"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres.npxfvhvukuvsqikxeyic:Prithvi17082003@aws-0-ap-south-1.pooler.supabase.com:6543/postgres?sslmode=require"
	}

	ctx := context.Background()
	database, err := db.ConnectSupabaseDB(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	pool := database.Pool

	// User: user276067
	userID := uuid.MustParse("9b4be774-bcd0-401b-95cf-96b083a500ec")
	// Listener: Barkha
	listenerID := uuid.MustParse("be8f845a-cc8b-4b95-8739-94622762de7f")

	tp := calls.NewZegoTokenProvider("628007464", "e7dffb8a9cb6a89f1fc2afddcc16f4ce4df9cd1e8ca346076161caf69cbd465e")
	ws := wallet.NewWalletService(pool)
	callSvc := calls.NewCallService(pool, tp, ws)

	fmt.Println("Testing InitiateCall...")
	initRes, err := callSvc.InitiateCall(ctx, userID, listenerID)
	if err != nil {
		log.Fatalf("InitiateCall error: %v", err)
	}
	fmt.Printf("✓ Call Initiated Successfully! SessionID: %s, RoomID: %s\n", initRes.SessionID, initRes.RoomID)

	fmt.Println("\nTesting GetIncomingCallForListener...")
	incoming, err := callSvc.GetIncomingCallForListener(ctx, listenerID)
	if err != nil {
		log.Fatalf("GetIncomingCall error: %v", err)
	}
	if incoming == nil {
		log.Fatalf("No incoming call found!")
	}
	fmt.Printf("✓ Incoming Call Found for Listener! Caller: %s (%s) | RoomID: %s | Status: %s | Earning/min: ₹%.2f\n",
		incoming.CallerName, incoming.CallerID, incoming.RoomID, incoming.Status, incoming.EarningPerMin)

	fmt.Println("\nTesting AcceptCall...")
	sessionUUID := uuid.MustParse(initRes.SessionID)
	acceptRes, err := callSvc.AcceptCall(ctx, sessionUUID, listenerID)
	if err != nil {
		log.Fatalf("AcceptCall error: %v", err)
	}
	fmt.Printf("✓ Call Accepted! RoomID: %s\n", acceptRes.RoomID)

	fmt.Println("\nTesting EndCall...")
	err = callSvc.EndCall(ctx, sessionUUID, userID, "user", "test_finished")
	if err != nil {
		log.Fatalf("EndCall error: %v", err)
	}
	fmt.Println("✓ Call Ended Successfully and Listener unlocked!")
}
