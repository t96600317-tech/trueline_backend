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
	-- 1. Clear old seed listeners
	DELETE FROM chat_messages;
	DELETE FROM listeners WHERE id IN (
		'a0000000-0000-0000-0000-000000000001',
		'a0000000-0000-0000-0000-000000000002',
		'a0000000-0000-0000-0000-000000000003',
		'a0000000-0000-0000-0000-000000000004',
		'a0000000-0000-0000-0000-000000000005',
		'a0000000-0000-0000-0000-000000000006'
	);

	-- 2. Insert verified active Listeners
	INSERT INTO listeners (
		id, phone_hash, encrypted_phone, name, title, photo_url, audio_sample_url, bio, languages,
		rate_per_min_micros, earning_per_min_micros, rating_avg, rating_count, onboarding_step, kyc_status, status, availability
	) VALUES 
	(
		'a0000000-0000-0000-0000-000000000001',
		'hash_afreen',
		'enc_afreen',
		'Afreen',
		'Joy Helper',
		'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Always here to listen and bring joy to your day. Specializing in daily life chats, stress relief, and friendly advice.',
		ARRAY['Hindi', 'Bengali'],
		9000000,
		4500000,
		4.9,
		48,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000002',
		'hash_ahmedi',
		'enc_ahmedi',
		'Ahmedi',
		'Calm Friend',
		'https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'A calm and patient listener. Let us talk about work pressure, personal goals, or just have a soothing conversation.',
		ARRAY['Hindi', 'Urdu'],
		9000000,
		4500000,
		4.8,
		54,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000003',
		'hash_saima',
		'enc_saima',
		'Saima',
		'Warm Heart',
		'https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Fluent in English and Hindi. I offer a safe, warm space for anyone feeling lonely or needing someone to vent to.',
		ARRAY['English', 'Hindi'],
		9000000,
		4500000,
		4.8,
		29,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000004',
		'hash_nirvi',
		'enc_nirvi',
		'Nirvi',
		'Happy Coach',
		'https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Positivity and happiness coach. Talk to me whenever you feel down or need motivation.',
		ARRAY['Hindi', 'Punjabi'],
		9000000,
		4500000,
		4.9,
		61,
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
		'Loves chatting in Bhojpuri and Hindi. Great for warm, family-style conversations.',
		ARRAY['Bhojpuri', 'Hindi'],
		9000000,
		4500000,
		4.7,
		19,
		'completed',
		'approved',
		'active',
		'online'
	),
	(
		'a0000000-0000-0000-0000-000000000006',
		'hash_kavitha',
		'enc_kavitha',
		'Kavitha M',
		'Life Buddy',
		'https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=400&q=80',
		'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
		'Friendly Tamil listener. Let us talk about cinema, everyday life, and peace of mind.',
		ARRAY['Tamil', 'English'],
		9000000,
		4500000,
		4.65,
		15,
		'completed',
		'approved',
		'active',
		'offline'
	);
	`

	_, err = conn.Exec(ctx, seedSQL)
	if err != nil {
		fmt.Printf("Seed error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Seed listeners created successfully!")
}
