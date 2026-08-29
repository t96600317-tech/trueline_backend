package main

import (
	"context"
	"fmt"
	"log"
	"os"

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

	rows, err := database.Pool.Query(ctx, `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = 'call_sessions'
		ORDER BY ordinal_position;
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("--- CALL_SESSIONS COLUMNS ---")
	for rows.Next() {
		var col, dtype string
		_ = rows.Scan(&col, &dtype)
		fmt.Printf("Column: %-30s | Type: %s\n", col, dtype)
	}
}
