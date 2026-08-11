-- name: CreateWallet :one
INSERT INTO wallets (user_id, balance)
VALUES ($1, $2)
RETURNING *;

-- name: GetWalletByUserID :one
SELECT * FROM wallets
WHERE user_id = $1;

-- name: UpdateWalletBalance :one
UPDATE wallets
SET balance = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: InsertWalletTransaction :one
INSERT INTO wallet_transactions (
    wallet_id, type, amount, balance_after, reference_id, idempotency_key, description
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM wallet_transactions
WHERE idempotency_key = $1;

-- name: ListWalletTransactions :many
SELECT * FROM wallet_transactions
WHERE wallet_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
