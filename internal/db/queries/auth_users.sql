-- name: CreateOTPRequest :one
INSERT INTO otp_requests (phone, otp_code, role, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestOTP :one
SELECT * FROM otp_requests
WHERE phone = $1 AND role = $2 AND verified = FALSE AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1;

-- name: MarkOTPVerified :exec
UPDATE otp_requests
SET verified = TRUE
WHERE id = $1;

-- name: FindUserByPhone :one
SELECT * FROM users
WHERE phone = $1;

-- name: CreateUser :one
INSERT INTO users (phone, language_pref)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateUserLanguage :one
UPDATE users
SET language_pref = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: FindPartnerByPhone :one
SELECT * FROM partners
WHERE phone = $1;

-- name: CreatePartner :one
INSERT INTO partners (phone, name, photo_url, bio, languages, rate_per_min)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetPartnerByID :one
SELECT * FROM partners
WHERE id = $1;

-- name: UpdatePartnerProfile :one
UPDATE partners
SET name = $2, photo_url = $3, bio = $4, languages = $5, rate_per_min = $6, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdatePartnerKYCStatus :one
UPDATE partners
SET kyc_status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdatePartnerAvailability :one
UPDATE partners
SET availability = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetPartnerCurrentCallSession :exec
UPDATE partners
SET current_call_session_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: ClearPartnerCurrentCallSession :exec
UPDATE partners
SET current_call_session_id = NULL, updated_at = NOW()
WHERE id = $1;

-- name: ListOnlinePartners :many
SELECT * FROM partners
WHERE status = 'active' AND kyc_status = 'approved' AND availability = 'online'
ORDER BY rating_avg DESC, created_at DESC;

-- name: ListAllPartners :many
SELECT * FROM partners
WHERE status = 'active' AND kyc_status = 'approved'
ORDER BY 
    CASE WHEN availability = 'online' THEN 1 ELSE 2 END,
    rating_avg DESC;
