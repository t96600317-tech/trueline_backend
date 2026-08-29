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

	restoreSQL := `
	-- 1. Insert User user276067
	INSERT INTO users (id, phone_hash, encrypted_phone, name, language_pref, status, created_at, updated_at)
	VALUES (
		'9b4be774-bcd0-401b-95cf-96b083a500ec',
		'hash_user276067',
		'enc_user276067',
		'user276067',
		'hi',
		'active',
		NOW() - INTERVAL '2 days',
		NOW()
	) ON CONFLICT (id) DO UPDATE SET updated_at = NOW(), name = 'user276067', status = 'active';

	-- 2. Insert Listener Barkha
	INSERT INTO listeners (
		id, phone_hash, encrypted_phone, name, title, photo_url, audio_sample_url, bio, languages,
		rate_per_min_micros, earning_per_min_micros, rating_avg, rating_count, onboarding_step, kyc_status, status, availability, created_at, updated_at
	) VALUES (
		'be8f845a-cc8b-4b95-8739-94622762de7f',
		'hash_barkha',
		'enc_barkha',
		'Barkha',
		'Empathetic Friend',
		'https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'I am here to listen with compassion, patience, and absolute confidentiality.',
		ARRAY['Hindi', 'English'],
		9000000,
		3000000,
		0.0,
		0,
		'approved',
		'approved',
		'active',
		'online',
		NOW() - INTERVAL '2 days',
		NOW()
	) ON CONFLICT (id) DO UPDATE SET status = 'active', kyc_status = 'approved', availability = 'online', updated_at = NOW();

	-- 3. Restore Wallets
	INSERT INTO wallets (user_id, balance_micros, updated_at)
	VALUES ('9b4be774-bcd0-401b-95cf-96b083a500ec', 500000000, NOW())
	ON CONFLICT (user_id) DO NOTHING;

	-- 4. Restore Chat Messages (Sent by listener)
	DELETE FROM chat_messages WHERE listener_id = 'be8f845a-cc8b-4b95-8739-94622762de7f';

	INSERT INTO chat_messages (id, user_id, listener_id, sender_type, content, moderation_status, read_at, created_at)
	VALUES 
	('c0000001-0000-0000-0000-000000000001', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'iiquhna', 'approved', NOW(), NOW() - INTERVAL '30 minutes'),
	('c0000001-0000-0000-0000-000000000002', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'ghuai77uhqbqh8ao', 'approved', NOW(), NOW() - INTERVAL '25 minutes'),
	('c0000001-0000-0000-0000-000000000003', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'hjwiwh', 'approved', NOW(), NOW() - INTERVAL '20 minutes'),
	('c0000001-0000-0000-0000-000000000004', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'iwii', 'approved', NOW(), NOW() - INTERVAL '15 minutes'),
	('c0000001-0000-0000-0000-000000000005', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'hieiwuuw', 'approved', NOW(), NOW() - INTERVAL '10 minutes'),
	('c0000001-0000-0000-0000-000000000006', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'jwiiuwhjwiwuhqjakp123', 'approved', NOW(), NOW() - INTERVAL '8 minutes'),
	('c0000001-0000-0000-0000-000000000007', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'hejwiuwh123', 'approved', NOW(), NOW() - INTERVAL '5 minutes'),
	('c0000001-0000-0000-0000-000000000008', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', '123', 'approved', NOW(), NOW() - INTERVAL '3 minutes'),
	('c0000001-0000-0000-0000-000000000009', '9b4be774-bcd0-401b-95cf-96b083a500ec', 'be8f845a-cc8b-4b95-8739-94622762de7f', 'listener', 'wo8eio1p0', 'approved', NOW(), NOW() - INTERVAL '1 minute')
	ON CONFLICT (id) DO UPDATE SET sender_type = 'listener';
	`

	_, err = pool.Exec(ctx, restoreSQL)
	if err != nil {
		log.Fatalf("Error restoring chat: %v", err)
	}

	fmt.Println("Successfully restored listener Barkha, user user276067, and all chat messages!")
}
