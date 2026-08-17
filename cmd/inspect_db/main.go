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
	_ = godotenv.Load(".env")
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `SELECT id, name, LEFT(photo_url, 100), LEFT(audio_sample_url, 100), encrypted_phone FROM listeners`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, photo, audio, phone string
		_ = rows.Scan(&id, &name, &photo, &audio, &phone)
		fmt.Printf("Listener: %s | Name: %s | Photo (prefix): %s | Audio (prefix): %s | EncPhone: %s\n", id, name, photo, audio, phone)
	}
}
