package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres.npxfvhvukuvsqikxeyic:Prithvi17082003@aws-0-ap-south-1.pooler.supabase.com:6543/postgres?sslmode=require&default_query_exec_mode=simple_protocol"
	}

	ctx := context.Background()
	config, err := pgx.ParseConfig(dbURL)
	if err != nil {
		log.Fatalf("ParseConfig failed: %v", err)
	}
	config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, config)
	if err != nil {
		log.Fatalf("Connect failed: %v", err)
	}
	defer conn.Close(ctx)

	var nowStr string
	_ = conn.QueryRow(ctx, "SELECT NOW()::text").Scan(&nowStr)
	fmt.Printf("Postgres NOW(): %s\n", nowStr)

	rows, err := conn.Query(ctx, `
		SELECT cs.id, cs.user_id, cs.listener_id, cs.room_id, cs.status, cs.created_at::text,
		       NOW() - cs.created_at as age
		FROM call_sessions cs
		ORDER BY cs.created_at DESC
		LIMIT 5;
	`)
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, uid, lid, room, status, created, age string
		_ = rows.Scan(&id, &uid, &lid, &room, &status, &created, &age)
		fmt.Printf("Session %s | User: %s | Listener: %s | Room: %s | Status: %s | Created: %s | Age: %s\n",
			id, uid, lid, room, status, created, age)
	}
}
