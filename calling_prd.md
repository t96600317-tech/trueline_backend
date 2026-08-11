# TrueLine — Calling Feature PRD

**Scope:** In-app VoIP calling via ZegoCloud, wallet billing integration, in-call UI, low-balance flow.
**Platforms:** Android + iOS, built with Kotlin Multiplatform (KMP). Shared logic in `commonMain`, native SDK wrapping in `androidMain`/`iosMain`.
**Provider:** ZegoCloud (1v1 Call UIKit — prebuilt UI, do not build call UI from scratch).
**Assumes:** Home and Chat screens are already built. This PRD covers Discover→Call, In-Call, Low-Balance, Post-Call, and the Partner incoming-call flow.

---

## 1. Goal

A user taps "Call" on a partner's profile/card. If they have enough balance, a VoIP call connects in under 10 seconds. The call bills per minute against the user's wallet in real time, warns at 2 minutes of balance remaining, and ends automatically and unilaterally when balance hits zero — with the server (never the client) as the source of truth for when that happens.

No phone numbers are ever exchanged or exposed, in call metadata, logs, or UI, at any layer.

---

## 2. Non-goals for v1

- Video calls, group calls
- Gifting / in-call monetary extras
- Call recording (pending open decision in main PRD — build the consent-notice hook now, gate the feature behind a flag so it can be turned on later without a re-architecture)
- Multi-device simultaneous calls (one active call per user, one active call per partner)

---

## 3. User-side flow

### 3.1 Call initiation (from Discover)

1. User taps call button on a partner card/profile.
2. Client calls `POST /calls/initiate` with `partner_id`.
3. Backend checks, in order:
   - Partner is online and not already on a call (see §6.2 concurrency lock)
   - User's wallet balance ≥ 1 minute at this partner's rate
   - User is not blocked by this partner / hasn't blocked them
4. If balance is insufficient → **do not call ZegoCloud at all** → return `402`-style error → client routes straight to Recharge screen (not the in-call low-balance sheet — that's a different state, see §3.3).
5. If checks pass → backend creates a `call_session` row (`status = pending`), requests a ZegoCloud room token, and sends a high-priority push to the partner (see §5).
6. Client receives session ID + Zego token, shows a "Calling…" state (ringing UI), joins the Zego room.
7. Partner accepts (§5) → both sides join the Zego room → backend flips `call_session.status = active`, records `started_at` server-side (this is the only timestamp billing is calculated from — never trust client-reported start time).
8. Target: ringing-to-connected under 10 seconds. If partner doesn't accept within a defined ring timeout (recommend 30s), backend cancels the session, no charge occurs, client returns to Discover.

### 3.2 In-call screen

Built as an **overlay on top of ZegoCloud's prebuilt call view**, not a full custom rebuild:

- Zego UIKit prebuilt view provides: live audio, mute toggle, speaker toggle, end-call button, connection-quality handling.
- App-owned overlay layer (rendered in KMP shared UI where the platform allows, native container otherwise) provides:
  - Partner name + photo (top, always visible)
  - Call timer (counts up from `started_at`, driven by local clock but reconciled against server balance pushes — see §4)
  - Balance remaining (in ₹, or minutes remaining — confirm with design; PRD assumes ₹ since that's what recharge packs show)
- End-call button: can be either Zego's native button wired to also call `POST /calls/{id}/end` (reason: `user_hangup`), or your own button that also triggers Zego's leave — either is fine, but the backend call **must** always be fired so the ledger closes correctly.

### 3.3 Low-balance warning (2 minutes remaining)

- Server-driven, not purely client-timer-driven (see §4.2 for exact trigger mechanism).
- At the 2-minutes-remaining threshold, server pushes a `low_balance` event to the client.
- Client shows a bottom sheet over the in-call screen:
  - Message: balance running low
  - One-tap recharge (pre-selected pack or the user's default pack — confirm with design/product which)
  - **Call continues uninterrupted** while the recharge payment is processing — do not pause billing, mute, or drop the Zego connection for this.
- On successful recharge:
  - Wallet balance updates server-side
  - Server pushes updated balance to client
  - Sheet auto-dismisses, timer/balance in the overlay reflects the new balance
- On failed/abandoned recharge:
  - Sheet can be dismissed by the user (X or swipe down)
  - Call continues normally; if balance actually reaches zero before another recharge attempt, §3.4 applies
  - Consider re-showing the sheet if balance drops further (e.g., at 1 minute) — confirm cadence with product, don't spam every 10 seconds

### 3.4 Balance hits zero → auto-end

- This is a **server-triggered** hangup, not client-triggered. The client does not decide to end the call based on its local timer reaching zero.
- Backend detects balance has hit zero (see §4.2), calls ZegoCloud's server-side room termination API, sets `call_session.status = ended`, `end_reason = low_balance`.
- Both clients receive a room-closed event from Zego + a push/socket event from backend confirming the reason, so the UI can show a specific "Call ended — balance depleted" message rather than a generic disconnect.
- Client routes user to Post-call screen with a nudge toward recharge.

### 3.5 Post-call

- Rating screen (1–5 stars), "Add to favourites" — these are existing designs per main PRD, not new for this feature, but note: the rating/favourite submission should carry `call_session_id` so it's tied to the specific call, and should not block on any billing finalization (billing should already be closed server-side by the time this screen shows).

---

## 4. Billing engine integration (client's role)

**The server is the sole source of truth for balance and call duration.** The client's timer/balance display is UX only — never authoritative, never used to decide call termination, never trusted for the final charge.

### 4.1 Real-time balance sync

- Separate channel from the Zego media session — use a WebSocket or SSE connection to your own backend, opened when the call starts, closed when it ends.
- Backend pushes balance updates on this channel at a defined cadence (recommend every 5–10 seconds, plus immediately on any billing tick or recharge event) — don't rely purely on the SDK's data-channel messaging for this; keep billing state transport independent of the call vendor.
- Client reconciles its local ticking timer against each pushed update — local timer is for smooth UI between pushes, server value always wins on conflict.

### 4.2 Server-side tick + thresholds

(Detailed in the Backend/Architecture PRD — call-session and billing-engine section. Summary of what the calling feature needs from it:)

- A per-minute billing tick that debits the wallet and can trigger three events: `low_balance` (at 2 min remaining), `balance_updated` (routine), `call_ended` (`end_reason = low_balance`).
- These three events are exactly what the calling UI (§3.2–3.4) is built to consume — coordinate field names/payload shape with backend implementers before building the client event handlers.

### 4.3 Dropped calls / network loss

- Zego reports disconnect/reconnect events — implement a reconnect grace window (recommend 15–20s) on the client: show "Reconnecting…" instead of immediately treating it as call-end.
- If reconnect succeeds within the window: call continues, billing was never paused (unless product wants to pause billing during confirmed disconnects — **open decision, flag for product**: does the user get charged for dead air during a drop?).
- If reconnect fails / window expires: client and backend both treat this as `end_reason = dropped`. Per main PRD money logic: **do not charge the incomplete minute** the drop occurred in — backend needs to handle this at the ledger level (see architecture PRD), calling feature just needs to make sure the client cleanly exits to post-call with the correct messaging ("Call dropped" vs "Call ended") rather than a generic error state.

---

## 5. Partner-side flow (incoming call)

- Must work when the partner's app is backgrounded or killed: **high-priority push (FCM/APNs) + native call UI** — CallKit on iOS, ConnectionService on Android. This is not optional; it's the only reliable way to get a full-screen incoming-call UI from a killed state on iOS in particular.
- Incoming call screen shows: caller name (never phone number), per-minute earning rate for this call.
- Accept → joins Zego room, mirrors the user-side connect flow (§3.1 step 7).
- Decline / timeout → backend cancels session (§3.1 step 8), no charge, partner returns to Home.
- **Concurrency lock**: a partner can only be on one call at a time. This must be enforced server-side at `POST /calls/initiate` time (checking partner's current call state before creating a new session/sending a new incoming-call push) — the calling feature's job is to handle the client-side result gracefully (if a partner goes from "online, available" to "on another call" between the user seeing the Discover list and tapping call, show a clear "partner is currently on another call" state rather than a silent failure).

---

## 6. KMP architecture for this feature

```
commonMain
 ├─ CallSessionRepository (expect/actual interface)
 │    fun initiateCall(partnerId): Flow<CallState>
 │    fun endCall(sessionId, reason)
 │    fun toggleMute(), toggleSpeaker()
 ├─ CallState (sealed class: Idle, Ringing, Connecting, Active(startedAt), LowBalance, Ended(reason))
 ├─ CallBillingSocket (expect/actual or shared Ktor client over WS/SSE)
 │    Flow<BalanceUpdate>, Flow<LowBalanceEvent>, Flow<CallEndedEvent>
 ├─ CallSessionApi (shared Ktor client — initiate/end REST calls)
 └─ InCallOverlayState (shared state holder consumed by both platform UIs — timer, balance, partner info)

androidMain
 └─ actual CallSessionRepository — wraps ZegoUIKitPrebuiltCallService, launches Zego's Activity/Compose
    entry point, listens to Zego SDK callbacks, translates into CallState

iosMain
 └─ actual CallSessionRepository — wraps ZegoUIKit iOS SDK via a thin Swift shim exposed through
    cinterop/XCFramework, presents Zego's UIViewController, translates callbacks into CallState
```

Key rule for implementers: **all cross-platform business logic (billing math, threshold checks, state transitions) lives in `commonMain`.** Platform code's only job is (a) driving the Zego SDK and (b) rendering the native call surface + overlay. If you find yourself writing threshold/balance logic inside `androidMain` or `iosMain`, that's a signal it belongs in the shared repository instead.

---

## 7. ZegoCloud-specific implementation notes

- Use **ZegoUIKit Prebuilt Call** (1-on-1 template), not the raw Zego Express Engine, per the "don't build call UI from scratch" directive — this gives you the connect/mute/speaker/end surface for free, while still exposing SDK callbacks/listeners for you to hook into `CallState`.
- Server needs a **ZegoCloud server-side integration** for:
  - Generating room tokens per call (never generate these client-side with a hardcoded secret)
  - Force-ending a room server-side (for the zero-balance auto-end in §3.4)
  - Receiving Zego webhooks for room/user join-leave events, to cross-check against your own `call_session` state (defense in depth against a client that dies without calling your `/calls/{id}/end` endpoint)
- Get exact webhook payload field names and the server SDK method for forced room termination from ZegoCloud's current docs before backend implementation starts — these are the two integration points most likely to have platform-specific quirks or versioned changes.

---

## 8. Edge cases checklist (for agents to explicitly handle, not just the happy path)

- [ ] Balance sufficient for <1 min but user still initiates → decide: allow a short call or block below a floor (recommend: require balance ≥ 1 full minute to initiate, per main PRD wording)
- [ ] Partner goes offline/on-another-call between Discover render and call tap
- [ ] User backgrounds the app mid-call (billing must continue server-side regardless of client foreground state)
- [ ] Recharge sheet shown, user recharges an amount that still isn't enough to clear the low-balance state (re-evaluate threshold, don't just dismiss the sheet)
- [ ] Double end-call race: user hits "end" at the same moment server auto-ends for zero balance — single state machine per session server-side prevents double-billing or duplicate end events
- [ ] App killed mid-call (not just backgrounded) — reconcile call state on next launch by checking `GET /calls/active` rather than assuming clean state
- [ ] Call recording flag (currently off) — build the consent-notice UI hook now but keep it feature-flagged off, per open decision in main PRD

---

## 9. Acceptance criteria

- Call connects within 10 seconds of partner accepting, on a normal network
- Balance display updates within 10s of any billing tick, recharge, or threshold event
- Low-balance sheet appears exactly once per threshold crossing (not repeatedly per tick) unless product specifies a re-show cadence
- Zero client-side authority over call termination for balance reasons — verified by testing with a modified/mocked client that ignores local balance and confirming the server still ends the call
- No phone number appears in any call-related UI, log, or push notification payload, on either side
