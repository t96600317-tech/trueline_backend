package admin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"trueline-backend/internal/auth"
	"trueline-backend/internal/db"
	"trueline-backend/internal/payouts"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminService struct {
	pool          *pgxpool.Pool
	queries       *db.Queries
	tokenManager  *auth.TokenManager
	payoutService *payouts.PayoutService
}

func NewAdminService(pool *pgxpool.Pool, tm *auth.TokenManager, ps *payouts.PayoutService) *AdminService {
	return &AdminService{
		pool:          pool,
		queries:       db.New(pool),
		tokenManager:  tm,
		payoutService: ps,
	}
}

type AdminStats struct {
	OnlineListenersCount int64   `json:"online_listeners_count"`
	ActiveCallsCount     int64   `json:"active_calls_count"`
	PendingKYCCount      int64   `json:"pending_kyc_count"`
	PendingPayoutsCount  int64   `json:"pending_payouts_count"`
	TotalRechargeINR     float64 `json:"total_recharge_inr"`
	TotalEarnedCoins     float64 `json:"total_earned_coins"`
}

type KYCReviewItem struct {
	ID              string   `json:"id"`
	ListenerID      string   `json:"listener_id"`
	ListenerName    string   `json:"listener_name"`
	Phone           string   `json:"phone"`
	Title           string   `json:"title"`
	DocumentType    string   `json:"document_type"`
	ProviderRef     string   `json:"provider_ref"`
	VerifiedName    string   `json:"verified_name"`
	Status          string   `json:"status"`
	AudioSampleURL  string   `json:"audio_sample_url"`
	AvatarURL       string   `json:"avatar_url"`
	Bio             string   `json:"bio"`
	Languages       []string `json:"languages"`
	CreatedAt       string   `json:"created_at"`
}

type AdminListenerItem struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	Title            string   `json:"title"`
	Languages        []string `json:"languages"`
	Bio              string   `json:"bio"`
	AudioSampleURL   string   `json:"audio_sample_url"`
	AvatarURL        string   `json:"avatar_url"`
	KYCStatus        string   `json:"kyc_status"`
	Availability     string   `json:"availability"`
	Status           string   `json:"status"` // 'active' | 'blocked'
	OnboardingStep   string   `json:"onboarding_step"`
	RatingAvg        float64  `json:"rating_avg"`
	RatingCount      int      `json:"rating_count"`
	RatePerMinCoins  float64  `json:"rate_per_min_coins"`
	EarnPerMinCoins  float64  `json:"earn_per_min_coins"`
	CreatedAt        string   `json:"created_at"`
}

type AdminUserItem struct {
	ID            string  `json:"id"`
	PhoneMasked   string  `json:"phone_masked"`
	Language      string  `json:"language"`
	WalletBalance float64 `json:"wallet_balance_coins"`
	CreatedAt     string  `json:"created_at"`
}

type AdminPayoutItem struct {
	ID             string  `json:"id"`
	ListenerID     string  `json:"listener_id"`
	ListenerName   string  `json:"listener_name"`
	AmountINR      float64 `json:"amount_inr"`
	TdsINR         float64 `json:"tds_inr"`
	NetINR         float64 `json:"net_inr"`
	UPIID          string  `json:"upi_id"`
	Status         string  `json:"status"`
	UPIRef         string  `json:"upi_ref"`
	RequestedAt    string  `json:"requested_at"`
	ProcessedAt    *string `json:"processed_at"`
}

type AdminLedgerItem struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	AccountType    string  `json:"account_type"` // "user_wallet" | "listener_earnings"
	EntityID       string  `json:"entity_id"`
	AmountCoins    float64 `json:"amount_coins"`
	BalanceAfter   float64 `json:"balance_after_coins"`
	ReferenceID    string  `json:"reference_id"`
	Description    string  `json:"description"`
	CreatedAt      string  `json:"created_at"`
}

func (s *AdminService) Login(ctx context.Context, email, password string) (string, error) {
	if s.pool == nil {
		// Development fallback
		if email == "admin@trueline.internal" && password == "AdminPilot2026!" {
			return s.tokenManager.GenerateToken(uuid.New(), "admin", "", 24*time.Hour)
		}
		return "", errors.New("database not connected")
	}

	var adminID pgtype.UUID
	var passwordHash string
	var role string

	query := `SELECT id, password_hash, role FROM admins WHERE email = $1`
	err := s.pool.QueryRow(ctx, query, email).Scan(&adminID, &passwordHash, &role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Check if demo admin account matches
			if email == "admin@trueline.internal" && password == "AdminPilot2026!" {
				return s.tokenManager.GenerateToken(uuid.New(), "admin", "", 24*time.Hour)
			}
			return "", errors.New("invalid email or password")
		}
		return "", err
	}

	if !auth.CheckPasswordHash(password, passwordHash) {
		return "", errors.New("invalid email or password")
	}

	token, err := s.tokenManager.GenerateToken(adminID.Bytes, role, "", 24*time.Hour)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *AdminService) GetStats(ctx context.Context) (*AdminStats, error) {
	if s.pool == nil {
		return &AdminStats{
			OnlineListenersCount: 4,
			ActiveCallsCount:     1,
			PendingKYCCount:      3,
			PendingPayoutsCount:  2,
			TotalRechargeINR:     12450.0,
			TotalEarnedCoins:     8300.0,
		}, nil
	}

	stats := &AdminStats{}
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM listeners WHERE availability = 'online' AND status = 'active'`).Scan(&stats.OnlineListenersCount)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM call_sessions WHERE status = 'active'`).Scan(&stats.ActiveCallsCount)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM kyc_requests WHERE status = 'pending'`).Scan(&stats.PendingKYCCount)
	_ = s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM payout_requests WHERE status = 'pending'`).Scan(&stats.PendingPayoutsCount)

	var totalPaise int64
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_paise), 0) FROM payments WHERE status = 'success'`).Scan(&totalPaise)
	stats.TotalRechargeINR = float64(totalPaise) / 100.0

	var totalEarnedMicros int64
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_micros), 0) FROM earnings_ledger WHERE type = 'call_credit'`).Scan(&totalEarnedMicros)
	stats.TotalEarnedCoins = float64(totalEarnedMicros) / 1_000_000.0

	return stats, nil
}

func (s *AdminService) ListKYCQueue(ctx context.Context) ([]KYCReviewItem, error) {
	if s.pool == nil {
		return []KYCReviewItem{
			{
				ID:             "kyc-001",
				ListenerID:     "lis-101",
				ListenerName:   "Pooja Sharma",
				DocumentType:   "pan",
				ProviderRef:    "ABCDE1234F",
				VerifiedName:   "POOJA SHARMA",
				Status:         "pending",
				AudioSampleURL: "https://example.com/audio1.mp3",
				Bio:            "Compassionate, active listener for stressful days.",
				Languages:      []string{"Hindi", "English"},
				CreatedAt:      time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	query := `
		SELECT DISTINCT ON (l.id)
			k.id, k.listener_id, l.name, COALESCE(l.title, ''), COALESCE(l.encrypted_phone, ''), k.document_type, COALESCE(k.provider_ref, ''), 
			COALESCE(k.verified_name, ''), k.status, COALESCE(l.audio_sample_url, ''), COALESCE(l.photo_url, ''), COALESCE(l.bio, ''), l.languages, k.created_at
		FROM kyc_requests k
		JOIN listeners l ON l.id = k.listener_id
		WHERE k.status = 'pending'
		ORDER BY l.id, k.created_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	encKey := os.Getenv("ENCRYPTION_KEY")
	list := make([]KYCReviewItem, 0)
	for rows.Next() {
		var item KYCReviewItem
		var id, listenerID pgtype.UUID
		var encPhone string
		var createdAt time.Time
		if err := rows.Scan(
			&id, &listenerID, &item.ListenerName, &item.Title, &encPhone, &item.DocumentType, &item.ProviderRef,
			&item.VerifiedName, &item.Status, &item.AudioSampleURL, &item.AvatarURL, &item.Bio, &item.Languages, &createdAt,
		); err != nil {
			return nil, err
		}
		item.ID = uuid.UUID(id.Bytes).String()
		item.ListenerID = uuid.UUID(listenerID.Bytes).String()
		item.CreatedAt = createdAt.Format(time.RFC3339)

		if encPhone != "" && encKey != "" {
			phone, err := auth.DecryptPhone(encPhone, encKey)
			if err == nil && phone != "" {
				item.Phone = phone
			} else {
				item.Phone = "Verified Mobile"
			}
		} else {
			item.Phone = "Verified Mobile"
		}

		list = append(list, item)
	}

	return list, nil
}

func (s *AdminService) ReviewKYC(ctx context.Context, kycID, adminID uuid.UUID, status, reason string) error {
	if s.pool == nil {
		return nil
	}

	if status != "approved" && status != "rejected" {
		return errors.New("invalid status: must be 'approved' or 'rejected'")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var listenerID pgtype.UUID
	err = tx.QueryRow(ctx, `SELECT listener_id FROM kyc_requests WHERE id = $1`, pgtype.UUID{Bytes: kycID, Valid: true}).Scan(&listenerID)
	if err != nil {
		return fmt.Errorf("kyc request not found: %w", err)
	}

	// 1. Update ALL pending KYC Requests for this listener
	_, err = tx.Exec(ctx, `
		UPDATE kyc_requests 
		SET status = $1, rejection_reason = $2, reviewed_by = $3, reviewed_at = NOW() 
		WHERE listener_id = $4 AND (status = 'pending' OR id = $5)
	`, status, reason, pgtype.UUID{Bytes: adminID, Valid: true}, listenerID, pgtype.UUID{Bytes: kycID, Valid: true})
	if err != nil {
		return err
	}

	if status == "rejected" {
		// Fetch phone_hash of the applicant
		var phoneHash string
		_ = tx.QueryRow(ctx, `SELECT phone_hash FROM listeners WHERE id = $1`, listenerID).Scan(&phoneHash)

		// Block listener account
		_, _ = tx.Exec(ctx, `
			UPDATE listeners 
			SET kyc_status = 'rejected', status = 'blocked', updated_at = NOW() 
			WHERE id = $1
		`, listenerID)

		if phoneHash != "" {
			// Block user account with same phone_hash
			_, _ = tx.Exec(ctx, `UPDATE users SET status = 'blocked', updated_at = NOW() WHERE phone_hash = $1`, phoneHash)

			// Ensure blocked_phones table exists and record phone_hash
			_, _ = tx.Exec(ctx, `
				CREATE TABLE IF NOT EXISTS blocked_phones (
					phone_hash TEXT PRIMARY KEY,
					reason TEXT NOT NULL DEFAULT 'KYC Application Rejected',
					blocked_by TEXT NOT NULL DEFAULT 'admin',
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)
			`)

			blockReason := reason
			if blockReason == "" {
				blockReason = "KYC Application Rejected by Admin"
			}
			_, _ = tx.Exec(ctx, `
				INSERT INTO blocked_phones (phone_hash, reason, blocked_by)
				VALUES ($1, $2, 'admin')
				ON CONFLICT (phone_hash) DO UPDATE SET reason = $2, created_at = NOW()
			`, phoneHash, blockReason)
		}
	} else {
		// 2. Approve Listener
		_, err = tx.Exec(ctx, `
			UPDATE listeners 
			SET kyc_status = 'approved', status = 'active', onboarding_step = 'approved', updated_at = NOW() 
			WHERE id = $1
		`, listenerID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (s *AdminService) ListListeners(ctx context.Context) ([]AdminListenerItem, error) {
	if s.pool == nil {
		return []AdminListenerItem{
			{
				ID:              "lis-101",
				Name:            "Pooja Sharma",
				Title:           "Mindfulness & Life Listener",
				Languages:       []string{"Hindi", "English"},
				Bio:             "Warm and non-judgmental support.",
				AudioSampleURL:  "https://example.com/audio.mp3",
				AvatarURL:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb?w=200",
				KYCStatus:       "approved",
				Availability:    "online",
				Status:          "active",
				RatingAvg:       4.9,
				RatingCount:     48,
				RatePerMinCoins: 9.0,
				EarnPerMinCoins: 3.0,
				CreatedAt:       time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	query := `
		SELECT id, name, title, languages, COALESCE(bio, ''), COALESCE(audio_sample_url, ''), COALESCE(photo_url, ''),
		       kyc_status, availability, status, onboarding_step,
		       COALESCE(rating_avg, 4.80), COALESCE(rating_count, 0), COALESCE(rate_per_min_micros, 9000000), COALESCE(earning_per_min_micros, 3000000), created_at
		FROM listeners
		ORDER BY created_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AdminListenerItem, 0)
	for rows.Next() {
		var item AdminListenerItem
		var id pgtype.UUID
		var ratingAvg pgtype.Numeric
		var rateMicros, earnMicros int64
		var createdAt time.Time

		if err := rows.Scan(
			&id, &item.Name, &item.Title, &item.Languages, &item.Bio, &item.AudioSampleURL, &item.AvatarURL,
			&item.KYCStatus, &item.Availability, &item.Status, &item.OnboardingStep, &ratingAvg, &item.RatingCount,
			&rateMicros, &earnMicros, &createdAt,
		); err != nil {
			return nil, err
		}

		rVal, _ := ratingAvg.Float64Value()
		item.ID = uuid.UUID(id.Bytes).String()
		item.RatingAvg = rVal.Float64
		item.RatePerMinCoins = float64(rateMicros) / 1_000_000.0
		item.EarnPerMinCoins = float64(earnMicros) / 1_000_000.0
		item.CreatedAt = createdAt.Format(time.RFC3339)
		list = append(list, item)
	}

	return list, nil
}

func (s *AdminService) ListUsers(ctx context.Context) ([]AdminUserItem, error) {
	if s.pool == nil {
		return []AdminUserItem{
			{
				ID:            "usr-001",
				PhoneMasked:   "+91 98765 43210",
				Language:      "Hindi",
				WalletBalance: 150.0,
				CreatedAt:     time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	query := `
		SELECT u.id, COALESCE(u.language_pref, 'hi'), COALESCE(w.balance_micros, 0), u.created_at
		FROM users u
		LEFT JOIN wallets w ON w.user_id = u.id
		ORDER BY u.created_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AdminUserItem, 0)
	for rows.Next() {
		var item AdminUserItem
		var id pgtype.UUID
		var balanceMicros int64
		var createdAt time.Time

		if err := rows.Scan(&id, &item.Language, &balanceMicros, &createdAt); err != nil {
			return nil, err
		}

		item.ID = uuid.UUID(id.Bytes).String()
		item.PhoneMasked = "+91 " + item.ID[:4] + "***"
		item.WalletBalance = float64(balanceMicros) / 1_000_000.0
		item.CreatedAt = createdAt.Format(time.RFC3339)
		list = append(list, item)
	}

	return list, nil
}

func (s *AdminService) ToggleListenerStatus(ctx context.Context, listenerID uuid.UUID, status string) error {
	if s.pool == nil {
		return nil
	}
	if status != "active" && status != "blocked" {
		return errors.New("invalid status: must be 'active' or 'blocked'")
	}

	_, err := s.pool.Exec(ctx, `UPDATE listeners SET status = $1, updated_at = NOW() WHERE id = $2`, status, pgtype.UUID{Bytes: listenerID, Valid: true})
	return err
}

func (s *AdminService) ListPayoutRequests(ctx context.Context) ([]AdminPayoutItem, error) {
	if s.pool == nil {
		return []AdminPayoutItem{
			{
				ID:           "pay-001",
				ListenerID:   "lis-101",
				ListenerName: "Pooja Sharma",
				AmountINR:    300.0,
				TdsINR:       30.0,
				NetINR:       270.0,
				UPIID:        "pooja@okhdfcbank",
				Status:       "pending",
				RequestedAt:  time.Now().Format(time.RFC3339),
			},
		}, nil
	}

	query := `
		SELECT p.id, p.listener_id, l.name, p.amount_micros, p.tds_micros, p.net_amount_micros, p.upi_id, p.status, COALESCE(p.upi_ref, ''), p.requested_at, p.processed_at
		FROM payout_requests p
		JOIN listeners l ON l.id = p.listener_id
		ORDER BY p.requested_at DESC
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AdminPayoutItem, 0)
	for rows.Next() {
		var item AdminPayoutItem
		var id, listenerID pgtype.UUID
		var amountMicros, tdsMicros, netMicros int64
		var reqAt time.Time
		var procAt pgtype.Timestamptz

		if err := rows.Scan(
			&id, &listenerID, &item.ListenerName, &amountMicros, &tdsMicros, &netMicros,
			&item.UPIID, &item.Status, &item.UPIRef, &reqAt, &procAt,
		); err != nil {
			return nil, err
		}

		item.ID = uuid.UUID(id.Bytes).String()
		item.ListenerID = uuid.UUID(listenerID.Bytes).String()
		item.AmountINR = float64(amountMicros) / 1_000_000.0
		item.TdsINR = float64(tdsMicros) / 1_000_000.0
		item.NetINR = float64(netMicros) / 1_000_000.0
		item.RequestedAt = reqAt.Format(time.RFC3339)
		if procAt.Valid {
			tStr := procAt.Time.Format(time.RFC3339)
			item.ProcessedAt = &tStr
		}

		list = append(list, item)
	}

	return list, nil
}

func (s *AdminService) ProcessPayout(ctx context.Context, requestID, adminID uuid.UUID, approve bool) error {
	if s.payoutService == nil {
		return errors.New("payout service not available")
	}
	return s.payoutService.ProcessPayout(ctx, requestID, adminID, approve)
}

func (s *AdminService) ListLedgers(ctx context.Context) ([]AdminLedgerItem, error) {
	if s.pool == nil {
		return []AdminLedgerItem{}, nil
	}

	query := `
		SELECT id, 'user_wallet' as account_type, type, wallet_id as entity_id, amount_micros, balance_after_micros, reference_id, description, created_at
		FROM wallet_ledger
		UNION ALL
		SELECT id, 'listener_earnings' as account_type, type, listener_id as entity_id, amount_micros, balance_after_micros, reference_id, '' as description, created_at
		FROM earnings_ledger
		ORDER BY created_at DESC
		LIMIT 100
	`
	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]AdminLedgerItem, 0)
	for rows.Next() {
		var item AdminLedgerItem
		var id, entityID pgtype.UUID
		var amountMicros, balanceMicros int64
		var createdAt time.Time

		if err := rows.Scan(
			&id, &item.AccountType, &item.Type, &entityID, &amountMicros, &balanceMicros,
			&item.ReferenceID, &item.Description, &createdAt,
		); err != nil {
			return nil, err
		}

		item.ID = uuid.UUID(id.Bytes).String()
		item.EntityID = uuid.UUID(entityID.Bytes).String()
		item.AmountCoins = float64(amountMicros) / 1_000_000.0
		item.BalanceAfter = float64(balanceMicros) / 1_000_000.0
		item.CreatedAt = createdAt.Format(time.RFC3339)
		list = append(list, item)
	}

	return list, nil
}
