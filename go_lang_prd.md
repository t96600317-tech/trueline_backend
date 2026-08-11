# TrueLine — Backend Implementation PRD (Go + Supabase + sqlc + net/http)

Companion doc to the Backend & System Architecture PRD. That doc defined *what* the backend needs to do (services, data model, API surface, flows). This doc defines *how* to build it with the specific stack: **Go, Supabase (Postgres), sqlc, standard library `net/http`.**

---

## 1. Stack summary and what each piece is actually for

| Piece | Role | Notes |
|---|---|---|
| Go | Application language | Per the stack decision in the Architecture PRD |
| Supabase | Managed Postgres (+ optionally Storage, Realtime) | Treat this primarily as **hosted Postgres with extras**, not as a BaaS you build the whole backend around — your Go service owns all business logic and writes. See §2 for what to use vs skip. |
| sqlc | Generates type-safe Go structs + query functions from raw SQL | No ORM. You write SQL, sqlc generates the Go code that runs it. |
| `net/http` (Go 1.22+) | HTTP routing and server | No framework (no Gin/Echo/Chi). Go 1.22's `http.ServeMux` supports method + path-parameter routing natively, which is enough for this API surface. |

This is a deliberately un-magical stack: SQL you can read, routing you can read, no hidden ORM query generation, no framework middleware stack to learn. That's the right call for a service where the highest-risk component is the money ledger — you want every query and every route to be something a reviewer can read top to bottom.

---

## 2. Supabase — what to use, what to skip

**Use:**

- **Postgres database** — this is the core of it. Supabase gives you a managed Postgres instance with connection pooling (PgBouncer) built in, which matters for a Go service holding many short-lived connections.
- **Storage** — for KYC document uploads (`kyc_documents.document_url`). Simpler than standing up your own S3-compatible bucket + signed URL logic from scratch.
- **Row-level backups / point-in-time recovery** — Supabase's managed backup story is a real advantage for a ledger-critical system; don't roll your own backup strategy on top of it, use what's provided.

**Skip (for v1):**

- **Supabase Auth** — you're doing phone+OTP with your own session/token model per the Architecture PRD, and your auth needs (separate User/Partner identity spaces, custom session semantics) don't map cleanly onto Supabase Auth's user model. Build auth in your Go service against your own `users`/`partners` tables. Revisit only if there's a specific reason to offload OTP delivery itself to a provider — that's a phone-OTP-vendor decision (e.g. MSG91, Twilio Verify), not an "auth backend" decision.
- **Supabase Realtime** (Postgres change-data-capture over websockets) — tempting for the balance-push feature, but resist it: the calling feature's `low_balance` / `balance_updated` / `call_ended` events need to be emitted by *your billing tick job* at the moment business logic decides they're true, not derived generically from a row changing. Build the WebSocket push yourself in the Go service (§6). Using Supabase Realtime here would mean two sources of truth for "what does the client get told and when."
- **Supabase Edge Functions / auto-generated REST (PostgREST) API** — skip entirely. All API access goes through your Go service. Exposing Postgres directly to clients bypasses the ledger discipline (idempotency keys, same-transaction tick+debit writes) that the Architecture PRD requires.

In short: Supabase is your Postgres host and file storage. Everything else is your Go service.

---

## 3. Project structure

```
trueline-backend/
├── cmd/
│   └── server/
│       └── main.go                 # wiring: config, db pool, routes, start server
├── internal/
│   ├── auth/
│   │   ├── handler.go
│   │   ├── service.go
│   │   └── middleware.go           # session validation middleware
│   ├── wallet/
│   │   ├── handler.go
│   │   ├── service.go              # ledger logic — the most heavily tested package in the repo
│   ├── calls/
│   │   ├── handler.go
│   │   ├── service.go              # call lifecycle, availability/concurrency lock
│   │   ├── billing.go              # per-minute tick engine
│   │   ├── zego.go                 # ZegoCloud server SDK/API wrapper
│   │   ├── ws.go                   # balance-push WebSocket handler
│   ├── partner/
│   │   ├── handler.go
│   │   └── service.go              # availability, earnings, payouts
│   ├── payments/
│   │   ├── handler.go
│   │   ├── razorpay.go             # or cashfree.go — aggregator-specific webhook + client
│   ├── admin/
│   │   ├── handler.go
│   │   └── service.go
│   ├── chat/
│   │   ├── handler.go
│   │   └── service.go
│   ├── db/
│   │   ├── sqlc.yaml                # sqlc config (or at repo root, see §4)
│   │   ├── migrations/              # SQL migration files (see §4)
│   │   ├── queries/                 # .sql files sqlc reads to generate code
│   │   └── generated/               # sqlc output — DO NOT hand-edit
│   ├── httpx/
│   │   ├── router.go                # ServeMux setup, route registration per domain
│   │   ├── middleware.go            # logging, recover, request-id, auth chaining
│   │   └── respond.go               # shared JSON response/error helpers
│   └── config/
│       └── config.go                # env var loading
├── go.mod
└── README.md
```

Each domain package (`auth`, `wallet`, `calls`, etc.) mirrors the service breakdown in the Architecture PRD §2 — that's intentional, so anyone cross-referencing the two docs finds the same boundaries.

---

## 4. Database layer: migrations + sqlc

### 4.1 Migrations

Use plain numbered SQL migration files (a lightweight tool like `golang-migrate` or `goose` — either is fine, pick one and standardize) against the Supabase Postgres instance:

```
internal/db/migrations/
  0001_users_partners.sql
  0002_wallets_ledger.sql
  0003_calls.sql
  0004_payments_payouts.sql
  0005_admin_chat_reports.sql
```

Tables map directly to the Architecture PRD §3 data model. A couple of implementation notes worth calling out for the migration authors:

- `wallet_transactions` must have a **unique constraint on `idempotency_key`** at the DB level — this is what actually prevents double-charges, not application-level checking alone (a retried webhook hitting the DB twice should fail on the unique constraint, not silently succeed twice).
- `wallets.balance` should have a comment/doc-note in the migration itself flagging it as a cache column, since a future engineer without context might be tempted to write directly to it.
- `partners.current_call_session_id` should be nullable with a foreign key to `call_sessions.id`, and the concurrency lock (§5.3) depends on this column being updated inside the same transaction as call-session creation.

### 4.2 sqlc

sqlc reads your migration schema + hand-written query files and generates typed Go. Example query file:

```sql
-- internal/db/queries/wallet.sql

-- name: GetWalletByUserID :one
SELECT * FROM wallets WHERE user_id = $1;

-- name: InsertWalletTransaction :one
INSERT INTO wallet_transactions (wallet_id, type, amount, balance_after, reference_id, idempotency_key)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdateWalletBalance :exec
UPDATE wallets SET balance = $2, updated_at = now() WHERE id = $1;
```

`sqlc.yaml` points at `migrations/` for schema and `queries/` for query files, outputs into `internal/db/generated`. Every domain package's `service.go` calls into `generated` — services never write raw SQL inline, and the `generated` package is never hand-edited (regenerate on every schema/query change via `sqlc generate`, commit the generated output to the repo so builds don't depend on running sqlc in CI... though also run `sqlc generate --dry-run`-equivalent check in CI to catch drift).

**Transactional writes** (e.g. billing tick: insert `call_billing_ticks` row + insert `wallet_transactions` row + update `wallets.balance`, all-or-nothing): sqlc generates per-query functions, so the transaction itself is composed in your Go service code using `pgx`'s transaction API (or `database/sql`'s, depending which driver you pick — `pgx` is the more common/performant pairing with sqlc for Postgres) wrapping multiple generated calls:

```go
func (s *WalletService) DebitForCallMinute(ctx context.Context, ...) error {
    tx, err := s.pool.Begin(ctx)
    // ... defer rollback
    q := s.queries.WithTx(tx)
    tick, err := q.InsertCallBillingTick(ctx, ...)
    txn, err := q.InsertWalletTransaction(ctx, ...)
    err = q.UpdateWalletBalance(ctx, ...)
    return tx.Commit(ctx)
}
```

This is the pattern for every money-touching write path (recharge credit, call debit, refund, admin adjustment) — no exceptions, no shortcut writes outside a transaction.

---

## 5. HTTP layer: `net/http` (no framework)

### 5.1 Routing

Go 1.22+'s `http.ServeMux` supports method + path parameters natively — sufficient for this API, no need for Chi/Gin:

```go
mux := http.NewServeMux()

mux.HandleFunc("POST /auth/otp/request", authHandler.RequestOTP)
mux.HandleFunc("POST /auth/otp/verify", authHandler.VerifyOTP)

mux.HandleFunc("GET /wallet", withAuth(walletHandler.GetBalance))
mux.HandleFunc("POST /wallet/recharge/initiate", withAuth(walletHandler.InitiateRecharge))
mux.HandleFunc("POST /wallet/recharge/webhook", paymentsHandler.RazorpayWebhook)

mux.HandleFunc("POST /calls/initiate", withAuth(callsHandler.Initiate))
mux.HandleFunc("POST /calls/{id}/accept", withAuth(callsHandler.Accept))
mux.HandleFunc("POST /calls/{id}/end", withAuth(callsHandler.End))
mux.HandleFunc("GET /calls/active", withAuth(callsHandler.GetActive))
mux.HandleFunc("GET /calls/{id}/stream", withAuth(callsHandler.BalanceStream)) // upgraded to WS inside handler
mux.HandleFunc("POST /webhooks/zegocloud", callsHandler.ZegoWebhook)

// ...partner, admin, chat routes follow the same pattern
```

Path params read via `r.PathValue("id")`.

### 5.2 Middleware

Standard library has no middleware chaining built in, so use the simple functional-wrapper pattern — no library needed for this:

```go
type Middleware func(http.HandlerFunc) http.HandlerFunc

func Chain(h http.HandlerFunc, mw ...Middleware) http.HandlerFunc {
    for i := len(mw) - 1; i >= 0; i-- {
        h = mw[i](h)
    }
    return h
}

// usage
mux.HandleFunc("POST /calls/initiate", Chain(callsHandler.Initiate, RequestID, Recover, Logger, RequireAuth))
```

Minimum middleware set: request ID injection, panic recovery, structured request logging, auth (session token → user/partner context), and for admin routes an additional `RequireAdminRole`.

### 5.3 Handler → service pattern

Handlers stay thin: parse request, call a service method, write response. All business logic (including the concurrency lock — checking and setting `partners.current_call_session_id` — and the balance/availability checks) lives in `service.go`, not in the handler, so it's unit-testable without spinning up HTTP.

```go
func (h *CallsHandler) Initiate(w http.ResponseWriter, r *http.Request) {
    userID := auth.UserIDFromContext(r.Context())
    var req InitiateCallRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httpx.Error(w, http.StatusBadRequest, "invalid_request")
        return
    }
    session, token, err := h.service.InitiateCall(r.Context(), userID, req.PartnerID)
    if errors.Is(err, calls.ErrInsufficientBalance) {
        httpx.Error(w, http.StatusPaymentRequired, "insufficient_balance")
        return
    }
    // ... other typed errors → status codes
    httpx.JSON(w, http.StatusOK, InitiateCallResponse{SessionID: session.ID, ZegoToken: token})
}
```

---

## 6. WebSocket balance push

`net/http` has no WebSocket support built in — this is the one place you need a small external dependency. Use `nhooyr.io/websocket` (or `gorilla/websocket` if the team prefers it — either is a reasonable, widely-used choice; pick one and standardize) purely for the `/calls/{id}/stream` endpoint. Nothing else in the stack needs a framework-level dependency for this.

Design:

- One WS connection per active call, opened by the client when the call starts (per Calling Feature PRD §4.1).
- Server side: the billing tick job (§7) publishes events (`balance_updated`, `low_balance`, `call_ended`) to an in-process pub/sub keyed by `call_session_id` (a simple `map[string]chan Event` guarded by a mutex, or a small pub/sub helper — no need for Redis at this scale for v1; revisit if you horizontally scale the service and need cross-instance fan-out).
- The WS handler for a given session subscribes to that channel and forwards events to the client as JSON frames.

---

## 7. Billing tick engine

This is the highest-risk piece of code in the service — treat it accordingly (extra tests, extra review, no cleverness).

**Recommended approach for v1:** one goroutine per active call, started when `call_session.status` flips to `active`, using a `time.Ticker` at your billing interval:

```go
func (s *CallsService) runBillingLoop(ctx context.Context, sessionID string) {
    ticker := time.NewTicker(1 * time.Minute) // or finer-grained if metering sub-minute internally
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): // call ended, context cancelled
            return
        case <-ticker.C:
            result, err := s.wallet.DebitForCallMinute(ctx, sessionID)
            if errors.Is(err, wallet.ErrInsufficientBalance) {
                s.endCall(ctx, sessionID, "low_balance")
                return
            }
            s.publish(sessionID, BalanceUpdatedEvent{...})
            if result.MinutesRemaining <= 2 {
                s.publish(sessionID, LowBalanceEvent{...})
            }
        }
    }
}
```

Cancel this goroutine's context on any call-end path (`user_hangup`, `dropped`, `low_balance`, admin force-end) so it never outlives the call. If the service restarts mid-call (deploy, crash), on startup reconcile against `call_sessions` where `status = active` and either resume billing loops or force-end orphaned sessions — decide which based on how long the service was down (a short deploy blip can resume; anything longer should probably force-end and let the client reconnect/re-initiate).

This in-process-goroutine approach is the right level of complexity for v1's scale. If you outgrow a single instance (need to horizontally scale the calls service), this becomes a job queue / distributed scheduler problem — don't build that now, note it as a scaling follow-up.

---

## 8. Config & environment

Simple struct populated from env vars (no config-management library needed):

```go
type Config struct {
    Port              string
    SupabaseDBUrl     string // includes pooler connection string
    SupabaseStorageURL string
    ZegoAppID         string
    ZegoServerSecret  string
    RazorpayKeyID     string
    RazorpayKeySecret string
    JWTSigningKey     string
    Env               string // local/staging/prod
}
```

Load via `os.Getenv` + a small validation pass at startup that fails fast if anything required is missing — don't let the service start in a half-configured state, especially for anything payments/billing related.

---

## 9. Testing strategy

- **Wallet/billing package**: the highest test-coverage bar in the repo. Table-driven tests covering: normal debit, insufficient balance, idempotency-key collision (retried webhook), concurrent debit attempts on the same wallet (race condition test using `go test -race`), refund/void path for dropped calls.
- **Calls package**: concurrency lock tests (two simultaneous `Initiate` calls to the same partner — only one should win), billing-loop cancellation on each end-reason path.
- **Integration tests**: spin up against a real (test) Postgres instance — Supabase offers branch/preview databases, or run a local Postgres via Docker for CI — rather than mocking the DB layer, since sqlc's generated code is a thin enough layer that mocking it tests very little of value. Test the actual SQL against a real schema.
- **HTTP layer**: table-driven handler tests using `httptest`, focused on status code / error mapping correctness rather than re-testing business logic already covered at the service layer.

---

## 10. Deployment notes

- Single Go binary, containerized (simple multi-stage Dockerfile — build stage + minimal runtime image).
- Connect to Supabase via its **pooled connection string** (PgBouncer), not the direct connection, given the number of concurrent short-lived queries from a service with many active WS connections and billing goroutines.
- Health check endpoint (`GET /healthz`) that checks DB connectivity — needed for whatever orchestrator/host you deploy to (not prescribing one here since it wasn't specified — Fly.io, Render, Railway, or a plain VM are all reasonable for this scale).
- Structured logging (JSON logs) from day one — given the ledger and billing correctness requirements, you want queryable logs for "what happened to this call session" without redeploying with extra debug statements later.

---

## 11. What this PRD deliberately leaves to the Architecture PRD

To avoid duplication/drift between docs, this PRD doesn't restate: the full data model (Architecture PRD §3), the full API surface (§4), or the end-to-end flows (§5) — it only adds the Go/Supabase/sqlc/net-http-specific *how*. If the two docs ever disagree on a field name or endpoint shape, the Architecture PRD is the source of truth for *what*, this one for *how* — flag the discrepancy rather than picking one silently.
