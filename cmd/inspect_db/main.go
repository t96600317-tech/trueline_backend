package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"io"
	"net/http"

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

	// 1. Delete fake chat messages pointing to dummy listener IDs
	_, _ = pool.Exec(ctx, `
		DELETE FROM chat_messages 
		WHERE listener_id::text LIKE 'a0000000%' 
		   OR user_id::text LIKE '11111111%' 
		   OR user_id::text LIKE '22222222%'
	`)

	// 2. Delete test listener records except Pranay Test1
	resListeners, err := pool.Exec(ctx, `
		DELETE FROM listeners 
		WHERE name IN ('Laxmi', 'Ritu', 'Home', 'Test Name', 'hjhjhjh')
		   OR name NOT LIKE '%Pranay%'
	`)
	if err != nil {
		log.Printf("Error deleting test listeners: %v", err)
	} else {
		fmt.Printf("Deleted %d test listeners.\n", resListeners.RowsAffected())
	}

	// 3. Delete fake users
	_, _ = pool.Exec(ctx, `
		DELETE FROM users 
		WHERE id::text LIKE '11111111-%' 
		   OR id::text LIKE '22222222-%'
	`)

	// 4. Ensure all real registered listeners have status='active' and kyc_status='approved'
	_, err = pool.Exec(ctx, `
		UPDATE listeners 
		SET status = 'active', kyc_status = 'approved', onboarding_step = 'approved', updated_at = NOW() 
		WHERE status != 'blocked'
	`)
	if err != nil {
		log.Printf("Error updating listeners: %v", err)
	}

	// 5. Query remaining real listeners
	rows, err := pool.Query(ctx, `
		SELECT id, name, status, kyc_status, onboarding_step, availability, rate_per_min_micros, rating_avg, rating_count 
		FROM listeners 
		ORDER BY CASE WHEN availability = 'online' THEN 1 ELSE 2 END, rating_avg DESC
	`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\n--- REMAINING REAL REGISTERED LISTENERS ---")
	count := 0
	for rows.Next() {
		count++
		var id, name, status, kycStatus, onboardingStep, availability string
		var rate int64
		var ratingAvg float64
		var ratingCount int
		_ = rows.Scan(&id, &name, &status, &kycStatus, &onboardingStep, &availability, &rate, &ratingAvg, &ratingCount)
		fmt.Printf("[%d] Name: %-16s | Availability: %-8s | KYC: %-9s | Status: %s | ID: %s\n", 
			count, name, availability, kycStatus, status, id)
	}

	// 6. Test GET /api/v1/listeners from api.truelineapp.in
	req, _ := http.NewRequest("GET", "https://api.truelineapp.in/api/v1/listeners", nil)
	resp, err := http.DefaultClient.Do(req)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Println("\n--- HTTP RESPONSE FROM api.truelineapp.in/api/v1/listeners (No Token) ---")
		fmt.Printf("Status: %d\nBody: %s\n", resp.StatusCode, string(body))
	}
}
