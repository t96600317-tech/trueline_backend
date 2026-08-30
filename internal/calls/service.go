package calls

import (
	"context"
	"errors"
	"fmt"
	"time"

	"trueline-backend/internal/db"
	"trueline-backend/internal/wallet"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CallService struct {
	pool          *pgxpool.Pool
	queries       *db.Queries
	tokenProvider *ZegoTokenProvider
	walletService *wallet.WalletService
}

func NewCallService(pool *pgxpool.Pool, tp *ZegoTokenProvider, ws *wallet.WalletService) *CallService {
	return &CallService{
		pool:          pool,
		queries:       db.New(pool),
		tokenProvider: tp,
		walletService: ws,
	}
}

type CallInitiateResponse struct {
	SessionID string `json:"session_id"`
	RoomID    string `json:"room_id"`
	UserToken string `json:"user_token"`
}

func (s *CallService) InitiateCall(ctx context.Context, userID, listenerID uuid.UUID) (*CallInitiateResponse, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	// 1. Transactional check and lock
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	// 2. Check listener availability and lock with SELECT ... FOR UPDATE
	var listener db.ListenerGenerated
	queryListener := `SELECT id, availability, kyc_status, rate_per_min_micros, earning_per_min_micros, current_call_session_id 
	                  FROM listeners WHERE id = $1 FOR UPDATE`
	err = tx.QueryRow(ctx, queryListener, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(
		&listener.ID, &listener.Availability, &listener.KycStatus, &listener.RatePerMinMicros, &listener.EarningPerMinMicros, &listener.CurrentCallSessionID,
	)
	if err != nil {
		return nil, fmt.Errorf("listener not found: %w", err)
	}

	if listener.KycStatus != "approved" {
		return nil, errors.New("listener is not approved")
	}

	if listener.CurrentCallSessionID.Valid {
		var oldSessionStatus string
		var oldSessionCreatedAt time.Time
		var oldUserID pgtype.UUID
		_ = tx.QueryRow(ctx, `SELECT status, created_at, user_id FROM call_sessions WHERE id = $1`, listener.CurrentCallSessionID).Scan(&oldSessionStatus, &oldSessionCreatedAt, &oldUserID)
		if oldSessionStatus == "ended" || oldSessionStatus == "cancelled" || oldSessionStatus == "pending" || oldUserID.Bytes == userID || time.Since(oldSessionCreatedAt) > 30*time.Second {
			// Cancel previous pending/superseded session and unlock
			_, _ = tx.Exec(ctx, `UPDATE call_sessions SET status = 'cancelled', end_reason = 'superseded' WHERE id = $1 AND status = 'pending'`, listener.CurrentCallSessionID)
			_, _ = tx.Exec(ctx, `UPDATE listeners SET current_call_session_id = NULL WHERE id = $1`, listener.ID)
			listener.CurrentCallSessionID.Valid = false
		} else {
			return nil, errors.New("listener is currently busy on another call")
		}
	}

	if listener.Availability != "online" {
		return nil, errors.New("listener is currently offline")
	}

	// 3. Check user balance (at least 1 minute = 9 coins = 9,000,000 micros)
	w, err := qtx.GetWalletByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("user wallet not found: %w", err)
	}
	if w.BalanceMicros < 9000000 {
		return nil, errors.New("insufficient balance: minimum 9 coins required to start call")
	}

	// 4. Check if user is already on an active call (auto-clear stale pending sessions older than 60s)
	_, _ = tx.Exec(ctx, `UPDATE call_sessions SET status = 'ended', end_reason = 'timeout' WHERE user_id = $1 AND status = 'pending' AND created_at < NOW() - INTERVAL '60 seconds'`, pgtype.UUID{Bytes: userID, Valid: true})
	var userActiveCallCount int64
	_ = tx.QueryRow(ctx, `SELECT COUNT(*) FROM call_sessions WHERE user_id = $1 AND status IN ('pending', 'active')`, pgtype.UUID{Bytes: userID, Valid: true}).Scan(&userActiveCallCount)
	if userActiveCallCount > 0 {
		return nil, errors.New("user is already on an active call session")
	}

	// 5. Create Call Session
	roomID := fmt.Sprintf("call_%s_%s", listenerID.String()[:8], userID.String()[:8])
	session, err := qtx.CreateCallSession(ctx, db.CreateCallSessionParams{
		UserID:                      pgtype.UUID{Bytes: userID, Valid: true},
		ListenerID:                  pgtype.UUID{Bytes: listenerID, Valid: true},
		Provider:                    "zegocloud",
		RoomID:                      roomID,
		RatePerMinMicrosSnapshot:    listener.RatePerMinMicros,
		EarningPerMinMicrosSnapshot: listener.EarningPerMinMicros,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// 6. Lock listener to this session and mark busy
	err = qtx.SetListenerCurrentCallSession(ctx, db.SetListenerCurrentCallSessionParams{
		ID:                   listener.ID,
		CurrentCallSessionID: session.ID,
	})
	if err != nil {
		return nil, err
	}
	_, _ = tx.Exec(ctx, "UPDATE listeners SET availability = 'busy', updated_at = NOW() WHERE id = $1", listener.ID)

	// Generate the Zego credential before committing call state. A bad Zego
	// configuration must not leave the listener marked busy with no usable call.
	token, err := s.tokenProvider.GenerateToken(userID.String(), roomID, 1*time.Hour)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &CallInitiateResponse{
		SessionID: session.ID.String(),
		RoomID:    roomID,
		UserToken: token,
	}, nil
}

type IncomingCallSession struct {
	ID            string  `json:"id"`
	RoomID        string  `json:"room_id"`
	CallerID      string  `json:"caller_id"`
	CallerName    string  `json:"caller_name"`
	Status        string  `json:"status"`
	RatePerMin    float64 `json:"rate_per_min"`
	EarningPerMin float64 `json:"earning_per_min"`
	CreatedAt     string  `json:"created_at"`
}

func (s *CallService) GetIncomingCallForListener(ctx context.Context, listenerID uuid.UUID) (*IncomingCallSession, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	query := `
		SELECT cs.id::text, cs.room_id, cs.user_id::text,
		       COALESCE(NULLIF(u.name, ''), 'user' || (100000 + (abs(hashtext(u.id::text)) % 900000))::text) as caller_name,
		       cs.status,
		       (COALESCE(cs.rate_per_min_micros_snapshot, 9000000)::float8 / 1000000.0),
		       (COALESCE(cs.earning_per_min_micros_snapshot, 3000000)::float8 / 1000000.0),
		       cs.created_at::text
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		WHERE cs.listener_id = $1
		  AND cs.status = 'pending'
		  AND cs.created_at >= NOW() - INTERVAL '120 seconds'
		ORDER BY cs.created_at DESC
		LIMIT 1;
	`

	var inc IncomingCallSession
	err := s.pool.QueryRow(ctx, query, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(
		&inc.ID, &inc.RoomID, &inc.CallerID, &inc.CallerName, &inc.Status, &inc.RatePerMin, &inc.EarningPerMin, &inc.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("GetIncomingCall query error: %w", err)
	}

	return &inc, nil
}

type CallAcceptResponse struct {
	RoomID        string `json:"room_id"`
	ListenerToken string `json:"listener_token"`
}

func (s *CallService) AcceptCall(ctx context.Context, sessionID, listenerID uuid.UUID) (*CallAcceptResponse, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	session, err := s.queries.GetCallSessionByID(ctx, pgtype.UUID{Bytes: sessionID, Valid: true})
	if err != nil {
		return nil, err
	}
	if session.ListenerID.Bytes != listenerID {
		return nil, errors.New("unauthorized: caller is not the assigned listener for this call")
	}
	if session.Status != "pending" {
		return nil, fmt.Errorf("call is not in pending state (status: %s)", session.Status)
	}

	// Generate the credential before changing the session state. This keeps a
	// transient Zego configuration error from accepting an unusable call.
	token, err := s.tokenProvider.GenerateToken(listenerID.String(), session.RoomID, 1*time.Hour)
	if err != nil {
		return nil, err
	}

	_, err = s.queries.UpdateCallSessionStatus(ctx, db.UpdateCallSessionStatusParams{
		ID:     session.ID,
		Status: "active",
	})
	if err != nil {
		return nil, err
	}

	return &CallAcceptResponse{
		RoomID:        session.RoomID,
		ListenerToken: token,
	}, nil
}

func (s *CallService) EndCall(ctx context.Context, sessionID uuid.UUID, callerID uuid.UUID, callerRole string, reason string) error {
	if s.pool == nil {
		return errors.New("database not connected")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	qtx := s.queries.WithTx(tx)

	session, err := qtx.GetCallSessionByID(ctx, pgtype.UUID{Bytes: sessionID, Valid: true})
	if err != nil {
		return err
	}

	// Verify participant ownership unless admin or internal system
	if callerRole != "admin" && callerRole != "system" {
		if session.UserID.Bytes != callerID && session.ListenerID.Bytes != callerID {
			return errors.New("unauthorized: caller is not a participant in this call")
		}
	}

	if session.Status == "ended" || session.Status == "cancelled" {
		return nil // Already ended
	}

	// 1. Update session status
	_, err = qtx.EndCallSession(ctx, db.EndCallSessionParams{
		ID:        session.ID,
		EndReason: pgtype.Text{String: reason, Valid: true},
	})
	if err != nil {
		return err
	}

	// 2. Clear listener lock and revert availability to online
	err = qtx.ClearListenerCurrentCallSession(ctx, session.ListenerID)
	if err != nil {
		return err
	}
	_, _ = tx.Exec(ctx, "UPDATE listeners SET availability = 'online', updated_at = NOW() WHERE id = $1 AND availability = 'busy'", session.ListenerID)
	_, _ = tx.Exec(ctx, "UPDATE listener_waitlist SET notified = TRUE WHERE listener_id = $1 AND notified = FALSE", session.ListenerID)

	return tx.Commit(ctx)
}

func (s *CallService) GetSession(ctx context.Context, sessionID uuid.UUID) (*db.CallSessionGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	session, err := s.queries.GetCallSessionByID(ctx, pgtype.UUID{Bytes: sessionID, Valid: true})
	if err != nil {
		return nil, err
	}
	return &session, nil
}
