-- name: InsertKYCDocument :one
INSERT INTO kyc_documents (
    partner_id, document_type, document_url, review_status
) VALUES (
    $1, $2, $3, 'pending'
) RETURNING *;

-- name: GetKYCDocumentsByPartnerID :many
SELECT * FROM kyc_documents
WHERE partner_id = $1
ORDER BY created_at DESC;

-- name: ListPendingKYC :many
SELECT k.*, p.name as partner_name, p.phone as partner_phone
FROM kyc_documents k
JOIN partners p ON k.partner_id = p.id
WHERE k.review_status = 'pending'
ORDER BY k.created_at ASC;

-- name: ReviewKYCDocument :one
UPDATE kyc_documents
SET review_status = $2, rejection_reason = $3, reviewed_by = $4, reviewed_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreatePayoutRequest :one
INSERT INTO payout_requests (
    partner_id, amount_requested, tds_deducted, net_amount, status, upi_id
) VALUES (
    $1, $2, $3, $4, 'pending', $5
) RETURNING *;

-- name: ListPartnerPayouts :many
SELECT * FROM payout_requests
WHERE partner_id = $1
ORDER BY requested_at DESC;

-- name: ListPendingPayouts :many
SELECT pr.*, p.name as partner_name, p.phone as partner_phone
FROM payout_requests pr
JOIN partners p ON pr.partner_id = p.id
WHERE pr.status = 'pending'
ORDER BY pr.requested_at ASC;

-- name: ProcessPayoutRequest :one
UPDATE payout_requests
SET status = $2, upi_ref = $3, processed_at = NOW(), processed_by = $4
WHERE id = $1
RETURNING *;

-- name: AddRating :one
INSERT INTO ratings (
    call_session_id, user_id, partner_id, stars, review_text
) VALUES (
    $1, $2, $3, $4, $5
) RETURNING *;

-- name: UpdatePartnerAverageRating :exec
UPDATE partners
SET rating_avg = (
    SELECT COALESCE(AVG(stars), 0.00) FROM ratings WHERE partner_id = $1
), rating_count = (
    SELECT COUNT(*) FROM ratings WHERE partner_id = $1
)
WHERE id = $1;
