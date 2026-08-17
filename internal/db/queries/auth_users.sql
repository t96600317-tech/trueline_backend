-- name: CreateOTPRequest :one
INSERT INTO otp_requests (phone_hash, otp_code, role, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetLatestOTP :one
SELECT * FROM otp_requests
WHERE phone_hash = $1 AND role = $2 AND verified = FALSE AND expires_at > NOW()
ORDER BY created_at DESC
LIMIT 1;

-- name: MarkOTPVerified :exec
UPDATE otp_requests
SET verified = TRUE
WHERE id = $1;

-- name: FindUserByPhoneHash :one
SELECT * FROM users
WHERE phone_hash = $1;

-- name: CreateUser :one
INSERT INTO users (phone_hash, encrypted_phone, language_pref)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE id = $1;

-- name: UpdateUserLanguage :one
UPDATE users
SET language_pref = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: FindListenerByPhoneHash :one
SELECT * FROM listeners
WHERE phone_hash = $1;

-- name: CreateListener :one
INSERT INTO listeners (phone_hash, encrypted_phone)
VALUES ($1, $2)
RETURNING *;

-- name: GetListenerByID :one
SELECT * FROM listeners
WHERE id = $1;

-- name: UpdateListenerOnboardingStep :one
UPDATE listeners
SET onboarding_step = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateListenerProfile :one
UPDATE listeners
SET name = $2, title = $3, bio = $4, languages = $5, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateListenerKYCStatus :one
UPDATE listeners
SET kyc_status = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: UpdateListenerAvailability :one
UPDATE listeners
SET availability = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: SetListenerCurrentCallSession :exec
UPDATE listeners
SET current_call_session_id = $2, updated_at = NOW()
WHERE id = $1;

-- name: ClearListenerCurrentCallSession :exec
UPDATE listeners
SET current_call_session_id = NULL, updated_at = NOW()
WHERE id = $1;

-- name: ListOnlineListeners :many
SELECT * FROM listeners
WHERE status = 'active' AND kyc_status = 'approved' AND availability = 'online'
ORDER BY rating_avg DESC, created_at DESC;

-- name: ListAllListeners :many
SELECT * FROM listeners
WHERE status = 'active' AND kyc_status = 'approved'
ORDER BY 
    CASE WHEN availability = 'online' THEN 1 ELSE 2 END,
    rating_avg DESC;
