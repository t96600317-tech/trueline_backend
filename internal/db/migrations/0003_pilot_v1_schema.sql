-- TrueLine Pilot V1 Schema Migration
-- This migration resets the schema for the Pilot V1 release.
-- It handles money as BIGINT (micro-coins/paise) and implements identity hardening.

-- DROP existing tables to ensure clean state for Pilot V1
DROP TABLE IF EXISTS admin_actions CASCADE;
DROP TABLE IF EXISTS reports CASCADE;
DROP TABLE IF EXISTS ratings CASCADE;
DROP TABLE IF EXISTS favourites CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TABLE IF EXISTS payout_requests CASCADE;
DROP TABLE IF EXISTS partner_earnings CASCADE;
DROP TABLE IF EXISTS call_billing_ticks CASCADE;
DROP TABLE IF EXISTS call_sessions CASCADE;
DROP TABLE IF EXISTS wallet_transactions CASCADE;
DROP TABLE IF EXISTS wallets CASCADE;
DROP TABLE IF EXISTS otp_requests CASCADE;
DROP TABLE IF EXISTS kyc_documents CASCADE;
DROP TABLE IF EXISTS admins CASCADE;
DROP TABLE IF EXISTS partners CASCADE;
DROP TABLE IF EXISTS listeners CASCADE; -- New name
DROP TABLE IF EXISTS users CASCADE;

-- 1. Identity & Users
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_hash TEXT UNIQUE NOT NULL, -- HMAC-SHA256 of E.164 phone for lookup
    encrypted_phone TEXT NOT NULL,   -- AES-GCM encrypted E.164 phone
    language_pref VARCHAR(10) NOT NULL DEFAULT 'hi',
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Listeners (Replaces Partners)
CREATE TABLE listeners (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_hash TEXT UNIQUE NOT NULL,
    encrypted_phone TEXT NOT NULL,
    name VARCHAR(100) NOT NULL DEFAULT '',
    title VARCHAR(100) NOT NULL DEFAULT 'Calm Friend',
    photo_url TEXT NOT NULL DEFAULT '',
    audio_sample_url TEXT NOT NULL DEFAULT '',
    bio TEXT NOT NULL DEFAULT '',
    languages TEXT[] NOT NULL DEFAULT ARRAY['Hindi'],
    rate_per_min_micros BIGINT NOT NULL DEFAULT 9000000, -- 9 coins * 1M
    earning_per_min_micros BIGINT NOT NULL DEFAULT 3000000, -- 3 coins * 1M
    rating_avg NUMERIC(3,2) NOT NULL DEFAULT 4.80,
    rating_count INT NOT NULL DEFAULT 0,
    onboarding_step VARCHAR(30) NOT NULL DEFAULT 'phone_input',
    kyc_status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (kyc_status IN ('pending', 'approved', 'rejected')),
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'blocked')),
    availability VARCHAR(20) NOT NULL DEFAULT 'offline' CHECK (availability IN ('online', 'offline')),
    current_call_session_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Admins
CREATE TABLE admins (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    name VARCHAR(100) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'admin' CHECK (role IN ('superadmin', 'admin', 'support')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 4. OTP Requests
CREATE TABLE otp_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_hash TEXT NOT NULL,
    otp_code VARCHAR(10) NOT NULL,
    role VARCHAR(20) NOT NULL CHECK (role IN ('user', 'listener')),
    expires_at TIMESTAMPTZ NOT NULL,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_otp_phone_hash_role ON otp_requests(phone_hash, role);

-- 5. Wallets & Ledger
CREATE TABLE wallets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID UNIQUE NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    balance_micros BIGINT NOT NULL DEFAULT 0 CHECK (balance_micros >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wallet_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id UUID NOT NULL REFERENCES wallets(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL CHECK (type IN ('recharge', 'call_debit', 'refund', 'admin_adjustment')),
    amount_micros BIGINT NOT NULL,
    balance_after_micros BIGINT NOT NULL,
    reference_id TEXT NOT NULL, -- e.g. Payment ID or Call ID
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Earnings Ledger (For Listeners)
CREATE TABLE earnings_ledger (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL CHECK (type IN ('call_credit', 'payout', 'refund_debit', 'admin_adjustment')),
    amount_micros BIGINT NOT NULL,
    balance_after_micros BIGINT NOT NULL,
    reference_id TEXT NOT NULL,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    tax_info JSONB NOT NULL DEFAULT '{}'::jsonb, -- Store TDS/GST snapshots
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. Call Sessions
CREATE TABLE call_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id),
    listener_id UUID NOT NULL REFERENCES listeners(id),
    provider VARCHAR(50) NOT NULL DEFAULT 'zegocloud',
    room_id VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active', 'ended', 'cancelled')),
    started_at TIMESTAMPTZ,
    ended_at TIMESTAMPTZ,
    end_reason VARCHAR(50),
    rate_per_min_micros_snapshot BIGINT NOT NULL,
    earning_per_min_micros_snapshot BIGINT NOT NULL,
    recording_url TEXT,
    recording_status VARCHAR(20) DEFAULT 'none',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE listeners
    ADD CONSTRAINT fk_listeners_current_call_session
    FOREIGN KEY (current_call_session_id)
    REFERENCES call_sessions(id)
    ON DELETE SET NULL;

-- 8. KYC & Payouts
CREATE TABLE kyc_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL DEFAULT 'cashfree',
    provider_ref TEXT,
    document_type VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    verified_name TEXT,
    verified_dob DATE,
    rejection_reason TEXT,
    reviewed_by UUID REFERENCES admins(id),
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payout_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    amount_micros BIGINT NOT NULL,
    tds_micros BIGINT NOT NULL DEFAULT 0,
    net_amount_micros BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'paid', 'rejected')),
    upi_id VARCHAR(255) NOT NULL,
    upi_ref VARCHAR(255),
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    processed_by UUID REFERENCES admins(id)
);

-- 9. Payments (Recharge)
CREATE TABLE payments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    aggregator VARCHAR(50) NOT NULL DEFAULT 'cashfree',
    aggregator_order_id VARCHAR(255) NOT NULL,
    aggregator_payment_id VARCHAR(255),
    amount_paise BIGINT NOT NULL, -- INR in paise
    coins_credited_micros BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'initiated',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 10. Operational Tables
CREATE TABLE jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    locked_until TIMESTAMPTZ,
    retry_count INT NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 11. Chat & Safety
CREATE TABLE chat_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    sender_type VARCHAR(20) NOT NULL CHECK (sender_type IN ('user', 'listener')),
    content TEXT NOT NULL,
    moderation_status VARCHAR(20) DEFAULT 'pending',
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ratings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    call_session_id UUID NOT NULL REFERENCES call_sessions(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
    stars INT NOT NULL CHECK (stars >= 1 AND stars <= 5),
    review_text TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
