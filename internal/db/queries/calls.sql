-- name: CreateCallSession :one
INSERT INTO call_sessions (
    user_id, listener_id, provider, room_id, status, rate_per_min_micros_snapshot, earning_per_min_micros_snapshot
) VALUES (
    $1, $2, $3, $4, 'pending', $5, $6
) RETURNING *;

-- name: GetCallSessionByID :one
SELECT * FROM call_sessions
WHERE id = $1;

-- name: GetActiveCallSessionByUserID :one
SELECT * FROM call_sessions
WHERE user_id = $1 AND status IN ('pending', 'active')
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveCallSessionByListenerID :one
SELECT * FROM call_sessions
WHERE listener_id = $1 AND status IN ('pending', 'active')
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateCallSessionStatus :one
UPDATE call_sessions
SET status = $2, started_at = COALESCE(started_at, NOW())
WHERE id = $1
RETURNING *;

-- name: EndCallSession :one
UPDATE call_sessions
SET status = 'ended', ended_at = NOW(), end_reason = $2
WHERE id = $1
RETURNING *;

-- name: InsertEarningsLedgerEntry :one
INSERT INTO earnings_ledger (
    listener_id, type, amount_micros, balance_after_micros, reference_id, idempotency_key, tax_info
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetEarningsLedgerByIdempotencyKey :one
SELECT * FROM earnings_ledger
WHERE idempotency_key = $1;

-- name: GetListenerEarningsSummary :one
SELECT
    COALESCE(SUM(CASE WHEN type = 'call_credit' THEN amount_micros ELSE 0 END), 0)::BIGINT as total_earned,
    COALESCE(SUM(CASE WHEN type = 'payout' THEN amount_micros ELSE 0 END), 0)::BIGINT as total_paid
FROM earnings_ledger
WHERE listener_id = $1;
