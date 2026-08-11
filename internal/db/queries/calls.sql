-- name: CreateCallSession :one
INSERT INTO call_sessions (
    user_id, partner_id, provider, room_id, zego_token_ref, status, rate_per_min_snapshot
) VALUES (
    $1, $2, $3, $4, $5, 'pending', $6
) RETURNING *;

-- name: GetCallSessionByID :one
SELECT * FROM call_sessions
WHERE id = $1;

-- name: GetActiveCallSessionByUserID :one
SELECT * FROM call_sessions
WHERE user_id = $1 AND status IN ('pending', 'active')
ORDER BY created_at DESC
LIMIT 1;

-- name: GetActiveCallSessionByPartnerID :one
SELECT * FROM call_sessions
WHERE partner_id = $1 AND status IN ('pending', 'active')
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

-- name: InsertCallBillingTick :one
INSERT INTO call_billing_ticks (
    call_session_id, minute_index, amount_debited, wallet_balance_after
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: InsertPartnerEarnings :one
INSERT INTO partner_earnings (
    partner_id, call_session_id, amount_earned, tds_deducted, net_amount
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: ListCallBillingTicks :many
SELECT * FROM call_billing_ticks
WHERE call_session_id = $1
ORDER BY minute_index ASC;
