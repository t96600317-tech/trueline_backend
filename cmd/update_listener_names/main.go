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

	// 1. Update listener 1 (online) to "Ahana" (Name #01 from user's 100 Modern Indian Girl Names list)
	_, err = pool.Exec(ctx, `
		UPDATE listeners 
		SET name = 'Ahana', 
		    title = 'Compassionate Listener • Delhi, Delhi',
		    photo_url = 'data:image/svg+xml;utf8,<svg xmlns=''http://www.w3.org/2000/svg'' viewBox=''0 0 320 400'' width=''320'' height=''400''><defs><linearGradient id=''g'' x1=''0%'' y1=''0%'' x2=''100%'' y2=''100%''><stop offset=''0%'' stop-color=''%23193E44''/><stop offset=''100%'' stop-color=''%230E2024''/></linearGradient></defs><rect width=''320'' height=''400'' fill=''url(%23g)''/><circle cx=''160'' cy=''150'' r=''65'' fill=''%236DA2C2''/><ellipse cx=''160'' cy=''310'' rx=''105'' ry=''85'' fill=''%232D6A6B''/><text x=''160'' y=''170'' font-family=''sans-serif'' font-size=''60'' font-weight=''bold'' fill=''white'' text-anchor=''middle''>A</text></svg>',
		    updated_at = NOW() 
		WHERE id = '494dd29d-eda3-49c3-92b3-4edb1c808715'
	`)
	if err != nil {
		log.Printf("Error updating listener 1: %v", err)
	} else {
		fmt.Println("Updated listener 1 to 'Ahana'")
	}

	// 2. Update listener 2 (offline) to "Kiara" (Name #39 from user's 100 Modern Indian Girl Names list)
	_, err = pool.Exec(ctx, `
		INSERT INTO listeners (
			id, phone_hash, encrypted_phone, name, title, photo_url, audio_sample_url, bio, languages,
			rate_per_min_micros, earning_per_min_micros, rating_avg, rating_count, onboarding_step, kyc_status, status, availability, updated_at
		) VALUES (
			'b0000000-0000-0000-0000-000000000002',
			'hash_kiara',
			'enc_kiara',
			'Kiara',
			'Compassionate Listener • Mumbai, Maharashtra',
			'data:image/svg+xml;utf8,<svg xmlns=''http://www.w3.org/2000/svg'' viewBox=''0 0 320 400'' width=''320'' height=''400''><defs><linearGradient id=''g'' x1=''0%'' y1=''0%'' x2=''100%'' y2=''100%''><stop offset=''0%'' stop-color=''%23193E44''/><stop offset=''100%'' stop-color=''%230E2024''/></linearGradient></defs><rect width=''320'' height=''400'' fill=''url(%23g)''/><circle cx=''160'' cy=''150'' r=''65'' fill=''%236DA2C2''/><ellipse cx=''160'' cy=''310'' rx=''105'' ry=''85'' fill=''%232D6A6B''/><text x=''160'' y=''170'' font-family=''sans-serif'' font-size=''60'' font-weight=''bold'' fill=''white'' text-anchor=''middle''>K</text></svg>',
			'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
			'Always here to listen with an open heart. Warm, friendly and non-judgmental conversations.',
			ARRAY['Hindi', 'English'],
			9000000,
			3000000,
			4.80,
			0,
			'approved',
			'approved',
			'active',
			'offline',
			NOW()
		)
		ON CONFLICT (id) DO UPDATE SET 
			name = 'Kiara',
			title = 'Compassionate Listener • Mumbai, Maharashtra',
			photo_url = 'data:image/svg+xml;utf8,<svg xmlns=''http://www.w3.org/2000/svg'' viewBox=''0 0 320 400'' width=''320'' height=''400''><defs><linearGradient id=''g'' x1=''0%'' y1=''0%'' x2=''100%'' y2=''100%''><stop offset=''0%'' stop-color=''%23193E44''/><stop offset=''100%'' stop-color=''%230E2024''/></linearGradient></defs><rect width=''320'' height=''400'' fill=''url(%23g)''/><circle cx=''160'' cy=''150'' r=''65'' fill=''%236DA2C2''/><ellipse cx=''160'' cy=''310'' rx=''105'' ry=''85'' fill=''%232D6A6B''/><text x=''160'' y=''170'' font-family=''sans-serif'' font-size=''60'' font-weight=''bold'' fill=''white'' text-anchor=''middle''>K</text></svg>',
			status = 'active',
			kyc_status = 'approved',
			onboarding_step = 'approved',
			availability = 'offline',
			updated_at = NOW();
	`)
	if err != nil {
		log.Printf("Error updating listener 2: %v", err)
	} else {
		fmt.Println("Updated/Inserted listener 2 as 'Kiara'")
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
