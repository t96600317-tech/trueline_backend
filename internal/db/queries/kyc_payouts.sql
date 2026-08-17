-- name: CreateKYCRequest :one
INSERT INTO kyc_requests (
    listener_id, provider, provider_ref, document_type, status
) VALUES (
    $1, $2, $3, $4, 'pending'
) RETURNING *;

-- name: GetKYCRequestsByListenerID :many
SELECT * FROM kyc_requests
WHERE listener_id = $1
ORDER BY created_at DESC;

-- name: ListPendingKYC :many
SELECT k.*, l.name as listener_name
FROM kyc_requests k
JOIN listeners l ON k.listener_id = l.id
WHERE k.status = 'pending'
ORDER BY k.created_at ASC;

-- name: UpdateKYCRequestStatus :one
UPDATE kyc_requests
SET status = $2, verified_name = $3, verified_dob = $4, rejection_reason = $5, reviewed_by = $6, reviewed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreatePayoutRequest :one
INSERT INTO payout_requests (
    listener_id, amount_micros, tds_micros, net_amount_micros, status, upi_id
) VALUES (
    $1, $2, $3, $4, 'pending', $5
) RETURNING *;

-- name: ListListenerPayouts :many
SELECT * FROM payout_requests
WHERE listener_id = $1
ORDER BY requested_at DESC;

-- name: ListPendingPayouts :many
SELECT pr.*, l.name as listener_name
FROM payout_requests pr
JOIN listeners l ON pr.listener_id = l.id
WHERE pr.status = 'pending'
ORDER BY pr.requested_at ASC;

-- name: ProcessPayoutRequest :one
UPDATE payout_requests
SET status = $2, upi_ref = $3, processed_at = NOW(), processed_by = $4
WHERE id = $1
RETURNING *;

-- name: AddRating :one
INSERT INTO ratings (
    call_session_id, user_id, listener_id, stars, review_text
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdateListenerAverageRating :exec
UPDATE listeners
SET rating_avg = (
    SELECT COALESCE(AVG(stars), 0.00) FROM ratings WHERE ratings.listener_id = $1
), rating_count = (
    SELECT COUNT(*) FROM ratings WHERE ratings.listener_id = $1
)
WHERE id = $1;
