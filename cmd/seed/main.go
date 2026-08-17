package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

func main() {
	dbURL := "postgresql://postgres.hsxpsbmeyfmquxlxgqyo:TrueLinePilot2026%21@aws-0-ap-northeast-2.pooler.supabase.com:6543/postgres?sslmode=require&default_query_exec_mode=exec"
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		fmt.Printf("Connect error: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(ctx)

	seedSQL := `
	-- 1. Insert 10 Realistic & Verified Listeners
	INSERT INTO listeners (
		id, phone_hash, encrypted_phone, name, title, photo_url, audio_sample_url, bio, languages,
		rate_per_min_micros, earning_per_min_micros, rating_avg, rating_count, onboarding_step, kyc_status, status, availability
	) VALUES 
	(
		'a0000000-0000-0000-0000-000000000001',
		'hash_afreen',
		'enc_afreen',
		'Afreen Khan',
		'Joy Helper',
		'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Always here to listen with an open heart. Specializing in daily stress, emotional support, and friendly advice.',
		ARRAY['Hindi', 'Bengali', 'English'],
		9000000,
		4500000,
		4.9,
		72,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000002',
		'hash_ahmedi',
		'enc_ahmedi',
		'Ahmedi Sheikh',
		'Calm Friend',
		'https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'A patient and peaceful listener. Let us talk about work anxiety, relationships, or just unwind peacefully.',
		ARRAY['Hindi', 'Urdu'],
		9000000,
		4500000,
		4.8,
		58,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000003',
		'hash_saima',
		'enc_saima',
		'Saima Parveen',
		'Warm Heart',
		'https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Warm, compassionate, and non-judgmental. Here for late-night thoughts and gentle conversations.',
		ARRAY['English', 'Hindi'],
		9000000,
		4500000,
		4.85,
		43,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000004',
		'hash_nirvi',
		'enc_nirvi',
		'Nirvi Chopra',
		'Happy Coach',
		'https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Your daily booster of positivity and cheer. Talk to me when you need a smile or motivation.',
		ARRAY['Hindi', 'Punjabi', 'English'],
		9000000,
		4500000,
		4.95,
		89,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000005',
		'hash_pooja',
		'enc_pooja',
		'Pooja Sharma',
		'Desi Companion',
		'https://images.unsplash.com/photo-1488426862026-3ee34a7d66df?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Dil se baat sunne wali dost. Bhojpuri aur Hindi me baatein karein bina kisi jhijhak ke.',
		ARRAY['Bhojpuri', 'Hindi'],
		9000000,
		4500000,
		4.75,
		34,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000006',
		'hash_kavitha',
		'enc_kavitha',
		'Kavitha Murali',
		'Life Buddy',
		'https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Friendly Tamil & English listener. Let us talk about cinema, everyday life, and peace of mind.',
		ARRAY['Tamil', 'English'],
		9000000,
		4500000,
		4.65,
		21,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000007',
		'hash_ananya',
		'enc_ananya',
		'Ananya Sen',
		'Soul Listener',
		'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Deep and thoughtful chats in Bengali and Hindi. A soothing voice when you need quiet comfort.',
		ARRAY['Bengali', 'Hindi'],
		9000000,
		4500000,
		4.9,
		37,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000008',
		'hash_swathi',
		'enc_swathi',
		'Swathi Reddy',
		'Peaceful Guide',
		'https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Empathetic Telugu listener. Let us share thoughts and find peace together.',
		ARRAY['Telugu', 'English'],
		9000000,
		4500000,
		4.8,
		18,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000009',
		'hash_tanvi',
		'enc_tanvi',
		'Tanvi Kulkarni',
		'Mindful Healer',
		'https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Marathi and Hindi conversations. Gentle guidance and compassionate listening for mental clarity.',
		ARRAY['Marathi', 'Hindi'],
		9000000,
		4500000,
		4.7,
		26,
		'completed',
		'approved',
		'active',
		'offline'
	)
	ON CONFLICT (id) DO UPDATE SET
		name = EXCLUDED.name,
		title = EXCLUDED.title,
		photo_url = EXCLUDED.photo_url,
		audio_sample_url = EXCLUDED.audio_sample_url,
		bio = EXCLUDED.bio,
		languages = EXCLUDED.languages,
		rate_per_min_micros = EXCLUDED.rate_per_min_micros,
		rating_avg = EXCLUDED.rating_avg,
		rating_count = EXCLUDED.rating_count,
		availability = EXCLUDED.availability;

	-- 2. Seed chat messages for all registered users so they see real conversations
	INSERT INTO chat_messages (id, user_id, listener_id, sender_type, content, created_at, read_at)
	SELECT 
		gen_random_uuid(),
		u.id,
		'a0000000-0000-0000-0000-000000000001',
		'listener',
		'Namaste! I am online now. Feel free to reach out whenever you want to talk.',
		NOW() - INTERVAL '15 minutes',
		NULL
	FROM users u
	ON CONFLICT DO NOTHING;

	INSERT INTO chat_messages (id, user_id, listener_id, sender_type, content, created_at, read_at)
	SELECT 
		gen_random_uuid(),
		u.id,
		'a0000000-0000-0000-0000-000000000002',
		'listener',
		'Hello! Hope you are having a peaceful evening. Here whenever you need a friendly ear.',
		NOW() - INTERVAL '45 minutes',
		NOW() - INTERVAL '30 minutes'
	FROM users u
	ON CONFLICT DO NOTHING;

	INSERT INTO chat_messages (id, user_id, listener_id, sender_type, content, created_at, read_at)
	SELECT 
		gen_random_uuid(),
		u.id,
		'a0000000-0000-0000-0000-000000000004',
		'listener',
		'Hey there! Keep smiling today 😊',
		NOW() - INTERVAL '2 hours',
		NOW() - INTERVAL '1 hour'
	FROM users u
	ON CONFLICT DO NOTHING;
	`

	_, err = conn.Exec(ctx, seedSQL)
	if err != nil {
		fmt.Printf("Seed error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Database populated with rich verified listeners and chat threads!")
}
