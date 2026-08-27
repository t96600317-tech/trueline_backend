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

	// 1. Check all call sessions
	rows, err := pool.Query(ctx, `
		SELECT cs.id, cs.user_id, cs.listener_id, cs.room_id, cs.status, cs.created_at,
		       u.name as user_name, l.name as listener_name
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		LEFT JOIN listeners l ON l.id = cs.listener_id
		ORDER BY cs.created_at DESC
		LIMIT 10;
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("\n--- RECENT CALL SESSIONS IN DB ---")
	found := false
	for rows.Next() {
		found = true
		var id, uid, lid, room, status, created string
		var uName, lName *string
		_ = rows.Scan(&id, &uid, &lid, &room, &status, &created, &uName, &lName)
		uN, lN := "NULL", "NULL"
		if uName != nil {
			uN = *uName
		}
		if lName != nil {
			lN = *lName
		}
		fmt.Printf("Session: %s | User: %s (%s) -> Listener: %s (%s) | Room: %s | Status: %s | Created: %s\n",
			id, uN, uid, lN, lid, room, status, created)
	}
	if !found {
		fmt.Println("No call sessions found in DB!")
	}

	// 2. Check listeners and their availability & current_call_session_id
	lrows, _ := pool.Query(ctx, `SELECT id, name, availability, status, kyc_status, current_call_session_id FROM listeners;`)
	defer lrows.Close()
	fmt.Println("\n--- LISTENERS IN DB ---")
	for lrows.Next() {
		var id, name, avail, status, kyc string
		var curSession *string
		_ = lrows.Scan(&id, &name, &avail, &status, &kyc, &curSession)
		cur := "NONE"
		if curSession != nil {
			cur = *curSession
		}
		fmt.Printf("Listener %s (%s): Avail=%s, Status=%s, KYC=%s, CurSession=%s\n", name, id, avail, status, kyc, cur)
	}
}
