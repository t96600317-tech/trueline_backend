# TrueLine — Backend & System Architecture PRD

Companion doc to the Calling Feature PRD. Covers tech stack decision, service breakdown, data models, API surface, and end-to-end app flows for auth, wallet, calling, and partner earnings.

---

## 1. Tech stack decision: Go vs Kotlin backend

**Recommendation: Go**, for this specific system. Reasoning below — include it so the team can push back if a factor is wrong.

| Factor | Go | Kotlin (Ktor) |
|---|---|---|
| Concurrency for per-call billing ticks + WebSocket balance pushes | Goroutines are cheap and this is exactly their sweet spot — thousands of concurrent long-lived connections (one WS per active call) with lightweight per-connection state | JVM threads/coroutines also handle this well, but the ops model (JVM tuning, GC pauses) is more to manage for the same workload |
| Shared code with the KMP app | None — different language | Some — could theoretically share DTOs/validation via a shared Kotlin module, but in practice this rarely pays off across a client/server boundary with different serialization and framework concerns; the saving is smaller than it looks on paper |
| Hiring pool (India market) | Strong and growing, especially for backend-focused roles | Strong, especially if the team already has Android/Kotlin engineers who could flex into backend |
| Ecosystem maturity for payments/webhooks (Razorpay/Cashfree SDKs, ZegoCloud server SDK) | Good — Go is a first-class citizen for most Indian payment gateways' server SDKs | Good — JVM has mature HTTP/webhook tooling too |
| Operational simplicity | Single static binary, low memory footprint, simple deploys | JVM startup/memory overhead is real but manageable; more mature APM tooling ecosystem |
| Ledger correctness (money code) | No inherent advantage from the language — this is a design discipline problem (append-only ledger, DB transactions, idempotency keys), not a language problem | Same |

**Bottom line:** the "share code with the KMP app" argument is the only real pull toward Kotlin, and it's weaker than it sounds — your backend and app have different concerns (DB transactions and webhook handling vs UI state and SDK wrapping) and sharing DTOs across that boundary tends to create coupling that slows both sides down rather than saving time. Go's concurrency model is a more direct fit for the always-on-connections shape of this system (live wallet pushes + call session state for every active call). If the team is already strongest in Kotlin and wants one language end-to-end, Kotlin/Ktor is a completely defensible choice too — this is a "pick based on team strength" level decision, not a "one is objectively wrong" one. Whoever's building it should confirm before writing code.

The rest of this PRD is written stack-agnostically (service boundaries, data models, API shapes) so it applies either way.

---

## 2. Service breakdown

- **Auth** — phone + OTP, session/token issuance, separate identity spaces for User app and Partner app
- **Profile** — user profile, partner profile + KYC status
- **Wallet & Ledger** — balance, recharge, ledger entries, refunds
- **Call Session & Billing** — call lifecycle, ZegoCloud integration, per-minute billing engine, real-time balance push
- **Partner Availability** — online/offline state, current-call lock
- **Payments** — Razorpay/Cashfree integration (recharge in, payout out), TDS/GST handling
- **Chat** — text messaging (free in v1) — not detailed deeply here since it's a simpler CRUD+realtime feature; call out only where it touches wallet/favourites
- **Admin** — partner approval, reports, account suspension, manual wallet adjustment (itself ledgered, never a raw balance edit)
- **Push/Notifications** — FCM + APNs, high-priority incoming-call push, favourite-online push

These can be modules within a modular monolith to start (recommended for v1 — you don't have the scale to justify microservices yet, and it slows down the "zero billing disputes in pilot" goal by adding distributed-transaction risk to the ledger). Split into services later if/when a specific one needs independent scaling (Call Session & Billing is the most likely first candidate).

---

## 3. Data model

```
users
  id, phone (encrypted at rest), language_pref, status (active/blocked), created_at

partners
  id, phone (encrypted at rest), name, photo_url, languages[], bio,
  rate_per_min, rating_avg, kyc_status (pending/approved/rejected), status (active/blocked),
  availability (online/offline), current_call_session_id (nullable — the concurrency lock), created_at

kyc_documents
  id, partner_id, document_type, document_url, review_status, reviewed_by (admin_id), reviewed_at

wallets
  id, user_id, balance, updated_at
  -- balance is a DERIVED/CACHED value; source of truth is wallet_transactions. Recompute-and-compare
  -- periodically as a consistency check.

wallet_transactions
  id, wallet_id, type (recharge / call_debit / refund / admin_adjustment),
  amount, balance_after, reference_id (call_session_id / payment_id / admin_action_id),
  idempotency_key, created_at
  -- APPEND-ONLY. No updates, no deletes. Every balance change is a row here, full stop.

call_sessions
  id, user_id, partner_id, provider (zegocloud), room_id, zego_token_ref,
  status (pending / active / ended), started_at, ended_at,
  end_reason (user_hangup / partner_hangup / low_balance / dropped / no_answer / error),
  rate_per_min_snapshot   -- lock the rate at call start; don't let live rate changes affect an in-progress call

call_billing_ticks
  id, call_session_id, minute_index, amount_debited, wallet_balance_after,
  created_at
  -- one row per billed minute; this is your audit trail for "why was I charged"

partner_earnings
  id, partner_id, call_session_id, amount_earned, tds_deducted, net_amount, created_at

payout_requests
  id, partner_id, amount_requested, tds_deducted, net_amount, status (pending/approved/paid/rejected),
  upi_ref, requested_at, processed_at, processed_by (admin_id)

payments (recharge, inbound)
  id, user_id, aggregator (razorpay/cashfree), aggregator_ref, amount, gst_amount,
  status (initiated/success/failed), created_at

reports
  id, reporter_type (user/partner), reporter_id, reported_id, reason, status (open/reviewed/actioned),
  created_at

favourites
  id, user_id, partner_id, created_at

ratings
  id, call_session_id, user_id, partner_id, stars, created_at

admin_actions
  id, admin_id, action_type (approve_kyc / suspend / wallet_adjust / process_payout / ...),
  target_type, target_id, details_json, created_at
```

**Ledger discipline (non-negotiable given "zero billing disputes" is a stated success metric):**

- `wallet_transactions` is append-only. `wallets.balance` is a cache that must always be reconstructable by summing transactions.
- Every write path that touches money (call billing tick, recharge success, refund, admin adjustment) must use an **idempotency key** so retried webhooks or duplicate requests can't double-charge.
- Call billing ticks and the corresponding wallet transaction should be written in the same DB transaction — a tick that debits without a ledger row (or vice versa) is exactly the class of bug that causes disputes.

---

## 4. API surface (representative, not exhaustive)

### Auth

- `POST /auth/otp/request {phone}`
- `POST /auth/otp/verify {phone, otp}` → session token

### Wallet

- `GET /wallet` → balance
- `POST /wallet/recharge/initiate {pack_id}` → aggregator payment session
- `POST /wallet/recharge/webhook` (aggregator → backend, verifies + credits ledger)
- `GET /wallet/transactions`

### Calls

- `POST /calls/initiate {partner_id}` → session_id, zego_token, or 402-style insufficient-balance error
- `POST /calls/{id}/accept` (partner side)
- `POST /calls/{id}/decline` (partner side)
- `POST /calls/{id}/end {reason}`
- `GET /calls/active` (client reconciliation after app relaunch)
- `WS /calls/{id}/stream` → server→client: `balance_updated`, `low_balance`, `call_ended`; used by the calling feature's `CallBillingSocket` (see Calling Feature PRD §4.1)
- `POST /webhooks/zegocloud` (Zego → backend: room/user join-leave, for cross-checking session state)

### Partner

- `POST /partner/availability {online|offline}`
- `GET /partner/earnings`
- `POST /partner/payout/request {amount}`
- `POST /partner/kyc/upload`

### Admin

- `POST /admin/partners/{id}/approve`
- `POST /admin/reports/{id}/action`
- `POST /admin/wallet/adjust {user_id, amount, reason}` (still writes a ledger row — see §3)
- `POST /admin/payouts/{id}/process`

### Chat (lighter detail — flag for whoever builds it to confirm realtime transport, WS reuse vs separate)

- `GET /chats`, `GET /chats/{partner_id}/messages`, `POST /chats/{partner_id}/messages`

---

## 5. End-to-end app flows

### 5.1 Onboarding → first call (target: under 3 minutes, per success criteria)

1. Phone + OTP (Auth)
2. Language selection
3. Discover screen loads partner list (already built — this PRD doesn't cover Discover UI, only its call-initiation hook)
4. Tap call → if balance insufficient, straight to Recharge (not a failed-call state)
5. Recharge (Razorpay/Cashfree) → webhook confirms → ledger credited → balance reflected
6. Retry call → Calling Feature PRD flow takes over

### 5.2 Call → billing → end (full detail in Calling Feature PRD, backend responsibilities summarized)

1. `POST /calls/initiate` — availability + balance + concurrency checks, creates `call_session`, requests Zego token, sends partner push
2. Partner accepts → `status = active`, `started_at` set server-side
3. Billing tick job runs per minute (or finer-grained internally, billed per started minute per main PRD): writes `call_billing_ticks` + `wallet_transactions` in one DB transaction, pushes `balance_updated` over WS
4. At 2-minutes-remaining threshold → push `low_balance`
5. At zero balance → force-end via Zego server API, set `status = ended, end_reason = low_balance`, push `call_ended`
6. On drop/reconnect-timeout → `end_reason = dropped`, backend must not charge the incomplete minute (refund/void that tick if it was already written before the drop was detected — depends on tick granularity, needs careful implementation to avoid a race between "tick fired" and "drop detected")
7. On call end (any reason) → compute `partner_earnings` row (rate × billed minutes × split, minus TDS)

### 5.3 Partner: approval → first earning → withdrawal (target: no support contact needed)

1. Partner onboarding + KYC upload
2. Admin manual approval (`kyc_status = approved`) — until this, partner cannot go online
3. Partner toggles online → `availability = online` (and must not already have `current_call_session_id` set)
4. Receives calls, earns per `call_sessions` → `partner_earnings`
5. Requests payout → `payout_requests` (pending) → admin approval (manual for v1) → UPI payout processed, `net_amount` shown after TDS

---

## 6. Non-functional requirements (tied to main PRD's success criteria)

- Call connect time < 10s from partner accept — mostly a Zego/network concern, but backend's `POST /calls/initiate` → token issuance path should add negligible latency (target: under 500ms for this call)
- Install → first paid call under 3 minutes — backend's OTP delivery speed and recharge webhook turnaround are the likely bottlenecks; monitor OTP delivery and Razorpay/Cashfree webhook latency specifically
- Zero billing disputes in pilot — covered by the ledger discipline in §3; also means: build an internal admin view of `call_billing_ticks` + `wallet_transactions` per session early, so support/admin can explain any charge without engineering involvement

---

## 7. Security & compliance notes

- Phone numbers encrypted at rest; never included in any call-related payload, log line, or push notification body visible to the other party (per Calling Feature PRD §1)
- Payment card/UPI details never touch your servers — aggregator-hosted checkout only, per main PRD
- TDS deduction logic on payouts and GST inclusion on recharge packs should live in one clearly-owned module — these are compliance-sensitive calculations worth isolating and unit-testing heavily, not scattering across the payments flow
- Admin actions (`admin_actions` table) should log enough detail to answer "who changed what and why" for any wallet adjustment or account action — this matters both for disputes and for basic operational trust once there's a support team using the admin panel

---

## 8. Open items to confirm before backend build starts

1. Billing tick granularity: is metering truly per-second internally with billing rounded up per started minute (as main PRD recommends), or per-minute throughout? This changes the tick job's implementation.
2. Exact low-balance re-show cadence if user dismisses the sheet without recharging (see Calling Feature PRD §3.3)
3. Whether a network drop pauses billing during the reconnect grace window, or billing continues and only the final incomplete minute is voided (Calling Feature PRD §4.3 — flagged there too, needs one answer that both PRDs' implementers agree on)
4. Final Go vs Kotlin call — this PRD recommends Go; needs sign-off from whoever's actually staffing the backend build
5. All five "open decisions" from the main TrueLine PRD (coin-to-rupee mapping, final split, call recording, launch platform order, trademark) — none of them block starting backend scaffolding (auth, wallet skeleton, base schema), but they do block finishing the billing engine and the calling feature's recording hook
