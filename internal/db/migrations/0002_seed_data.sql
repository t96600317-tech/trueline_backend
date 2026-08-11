-- TrueLine Seed Data Migration 0002: Realistic Test Data for Mobile App Development

-- Clear old seed data if re-running
DELETE FROM chat_messages;
DELETE FROM favourites;
DELETE FROM partners;
DELETE FROM users;

-- 1. Insert Test Users
INSERT INTO users (id, phone, language_pref, status)
VALUES 
    ('11111111-1111-1111-1111-111111111111', '+919876543210', 'hi', 'active'),
    ('22222222-2222-2222-2222-222222222222', '+919123456789', 'en', 'active');

-- 2. Insert User Wallet with 260 Coins (matching design header!)
INSERT INTO wallets (user_id, balance)
VALUES 
    ('11111111-1111-1111-1111-111111111111', 260.00),
    ('22222222-2222-2222-2222-222222222222', 150.00)
ON CONFLICT (user_id) DO UPDATE SET balance = EXCLUDED.balance;

-- 3. Insert Test Listener Partners (matching UI screenshot details)
INSERT INTO partners (id, phone, name, title, photo_url, audio_sample_url, bio, languages, rate_per_min, rating_avg, rating_count, kyc_status, availability)
VALUES 
    (
        'a0000000-0000-0000-0000-000000000001',
        '+919000000001',
        'Afreen',
        'Joy Helper',
        'https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'Always here to listen and bring joy to your day. Specializing in daily life chats, stress relief, and friendly advice.',
        ARRAY['Hindi', 'Bengali'],
        11.00,
        4.50,
        38,
        'approved',
        'online'
    ),
    (
        'a0000000-0000-0000-0000-000000000002',
        '+919000000002',
        'Ahmedi',
        'Calm Friend',
        'https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'A calm and patient listener. Let us talk about work pressure, personal goals, or just have a soothing conversation.',
        ARRAY['Urdu', 'Hindi'],
        11.00,
        4.80,
        54,
        'approved',
        'online'
    ),
    (
        'a0000000-0000-0000-0000-000000000003',
        '+919000000003',
        'Saima',
        'Calm Friend',
        'https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'Fluent in English and Spanish. I offer a safe, warm space for anyone feeling lonely or needing someone to vent to.',
        ARRAY['English', 'Spanish'],
        11.00,
        4.80,
        29,
        'approved',
        'online'
    ),
    (
        'a0000000-0000-0000-0000-000000000004',
        '+919000000004',
        'Nirvi',
        'Happy Coach',
        'https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'Positivity and happiness coach. Talk to me whenever you feel down or need motivation.',
        ARRAY['English', 'Hindi'],
        11.00,
        4.90,
        61,
        'approved',
        'online'
    ),
    (
        'a0000000-0000-0000-0000-000000000005',
        '+919000000005',
        'Pooja Sharma',
        'Desi Companion',
        'https://images.unsplash.com/photo-1488426862026-3ee34a7d66df?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'Loves chatting in Bhojpuri and Hindi. Great for warm, family-style conversations.',
        ARRAY['Bhojpuri', 'Hindi'],
        11.00,
        4.70,
        19,
        'approved',
        'online'
    ),
    (
        'a0000000-0000-0000-0000-000000000006',
        '+919000000006',
        'Kavitha M',
        'Life Buddy',
        'https://images.unsplash.com/photo-1544005313-94ddf0286df2?auto=format&fit=crop&w=400&q=80',
        'https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg',
        'Friendly Tamil listener. Let us talk about cinema, everyday life, and peace of mind.',
        ARRAY['Tamil', 'English'],
        11.00,
        4.65,
        15,
        'approved',
        'offline'
    );

-- 4. Add User Favourites
INSERT INTO favourites (user_id, partner_id)
VALUES 
    ('11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000001'),
    ('11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000002');

-- 5. Seed Test 1-on-1 Chat Messages
INSERT INTO chat_messages (id, user_id, partner_id, sender_type, content, created_at, read_at)
VALUES 
    (gen_random_uuid(), '11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000001', 'user', 'Namaste Afreen! Are you free to talk today?', NOW() - INTERVAL '2 hours', NOW() - INTERVAL '1 hour 50 minutes'),
    (gen_random_uuid(), '11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000001', 'partner', 'Haan bilkul! Feel free to call anytime.', NOW() - INTERVAL '1 hour 45 minutes', NOW() - INTERVAL '1 hour 40 minutes'),
    (gen_random_uuid(), '11111111-1111-1111-1111-111111111111', 'a0000000-0000-0000-0000-000000000002', 'partner', 'Hello! Hope you had a peaceful day.', NOW() - INTERVAL '30 minutes', NULL);
