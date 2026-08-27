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
	pool := database.Pool

	// 1. Update listener 1 (Ahana) title to "Empathetic Friend • Delhi"
	_, err = pool.Exec(ctx, `
		UPDATE listeners 
		SET title = 'Empathetic Friend • Delhi',
		    updated_at = NOW() 
		WHERE id = '494dd29d-eda3-49c3-92b3-4edb1c808715'
	`)
	if err != nil {
		log.Printf("Error updating Ahana title: %v", err)
	} else {
		fmt.Println("Updated Ahana title to 'Empathetic Friend • Delhi'")
	}

	// 2. Update listener 2 (Kiara) title to "Caring Confidante • Mumbai"
	_, err = pool.Exec(ctx, `
		UPDATE listeners 
		SET title = 'Caring Confidante • Mumbai',
		    updated_at = NOW() 
		WHERE id = 'b0000000-0000-0000-0000-000000000002'
	`)
	if err != nil {
		log.Printf("Error updating Kiara title: %v", err)
	} else {
		fmt.Println("Updated Kiara title to 'Caring Confidante • Mumbai'")
	}

	// 3. Print current active listeners in DB
	rows, err := pool.Query(ctx, `
		SELECT id, name, title, availability, kyc_status, status, rate_per_min_micros, rating_avg, rating_count 
		FROM listeners 
		WHERE status = 'active' AND kyc_status = 'approved'
		ORDER BY CASE WHEN availability = 'online' THEN 1 ELSE 2 END, rating_avg DESC
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\n--- CURRENT ACTIVE LISTENERS IN DB ---")
	for rows.Next() {
		var id, name, title, availability, kycStatus, status string
		var rate int64
		var ratingAvg float64
		var ratingCount int
		_ = rows.Scan(&id, &name, &title, &availability, &kycStatus, &status, &rate, &ratingAvg, &ratingCount)
		fmt.Printf("ID: %s | Name: %-16s | Title: %-42s | Status: %-7s | Availability: %s\n", 
			id, name, title, status, availability)
	}
}
