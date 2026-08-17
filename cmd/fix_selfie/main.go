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
		log.Fatal("DATABASE_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to DB: %v", err)
	}
	defer pool.Close()

	// High quality demo SVG/JPEG portrait data URI
	samplePhoto := "data:image/svg+xml;utf8,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 320 400' width='320' height='400'><defs><linearGradient id='g' x1='0%' y1='0%' x2='100%' y2='100%'><stop offset='0%' stop-color='%23193E44'/><stop offset='100%' stop-color='%230E2024'/></linearGradient></defs><rect width='320' height='400' fill='url(%23g)'/><circle cx='160' cy='150' r='65' fill='%236DA2C2'/><ellipse cx='160' cy='310' rx='105' ry='85' fill='%232D6A6B'/><text x='160' y='170' font-family='sans-serif' font-size='60' font-weight='bold' fill='white' text-anchor='middle'>P</text></svg>"

	res, err := pool.Exec(ctx, `
		UPDATE listeners 
		SET photo_url = $1 
		WHERE photo_url IS NULL OR photo_url = '' OR photo_url LIKE '/data/%'
	`, samplePhoto)
	if err != nil {
		log.Fatalf("Failed to update photo_url: %v", err)
	}

	fmt.Printf("Updated listeners photo_url: %d rows affected\n", res.RowsAffected())
}
