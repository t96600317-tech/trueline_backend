-- name: CreateWallet :one
INSERT INTO wallets (user_id, balance_micros)
VALUES ($1, $2)
RETURNING *;

-- name: GetWalletByUserID :one
SELECT * FROM wallets
WHERE user_id = $1;

-- name: UpdateWalletBalance :one
UPDATE wallets
SET balance_micros = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: InsertWalletLedgerEntry :one
INSERT INTO wallet_ledger (
    wallet_id, type, amount_micros, balance_after_micros, reference_id, idempotency_key, description
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetWalletLedgerByIdempotencyKey :one
SELECT * FROM wallet_ledger
WHERE idempotency_key = $1;

-- name: ListWalletLedger :many
SELECT * FROM wallet_ledger
WHERE wallet_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
