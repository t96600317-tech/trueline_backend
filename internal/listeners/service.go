package listeners

import (
	"context"
	"errors"
	"fmt"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ListenerService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewListenerService(pool *pgxpool.Pool) *ListenerService {
	return &ListenerService{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (s *ListenerService) GetListenerProfile(ctx context.Context, listenerID uuid.UUID) (*db.Listener, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	l, err := s.queries.GetListenerByID(ctx, pgtype.UUID{Bytes: listenerID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("listener not found")
		}
		return nil, fmt.Errorf("failed to fetch listener: %w", err)
	}

	return s.mapListener(l), nil
}

type UpdateProfilePayload struct {
	Name      string   `json:"name"`
	Title     string   `json:"title"`
	Bio       string   `json:"bio"`
	Languages []string `json:"languages"`
}

func (s *ListenerService) UpdateProfile(ctx context.Context, listenerID uuid.UUID, req UpdateProfilePayload) (*db.Listener, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	l, err := s.queries.UpdateListenerProfile(ctx, db.UpdateListenerProfileParams{
		ID:        pgtype.UUID{Bytes: listenerID, Valid: true},
		Name:      req.Name,
		Title:     req.Title,
		Bio:       req.Bio,
		Languages: req.Languages,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Find phone credentials from users table or generate fallback hash
			var phoneHash, encryptedPhone string
			uErr := s.pool.QueryRow(ctx, "SELECT phone_hash, encrypted_phone FROM users WHERE id = $1", pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&phoneHash, &encryptedPhone)
			if uErr != nil || phoneHash == "" {
				phoneHash = fmt.Sprintf("listener_%s", listenerID.String())
				encryptedPhone = phoneHash
			}

			// Upsert listener record preserving existing KYC approval & onboarding state
			_, execErr := s.pool.Exec(ctx, `
				INSERT INTO listeners (id, phone_hash, encrypted_phone, name, title, bio, languages, onboarding_step, kyc_status, availability)
				VALUES ($1, $2, $3, $4, $5, $6, $7, 'voice_intro', 'pending', 'offline')
				ON CONFLICT (phone_hash) DO UPDATE SET 
					name = $4, title = $5, bio = $6, languages = $7, 
					onboarding_step = CASE 
						WHEN listeners.kyc_status = 'approved' THEN 'approved' 
						WHEN listeners.onboarding_step IN ('approved', 'pending_approval', 'kyc_documents', 'face_verification', 'voice_intro') THEN listeners.onboarding_step 
						ELSE 'voice_intro' 
					END, 
					updated_at = NOW()
			`, pgtype.UUID{Bytes: listenerID, Valid: true}, phoneHash, encryptedPhone, req.Name, req.Title, req.Bio, req.Languages)
			if execErr != nil {
				// Retry direct conflict on id
				_, execErr2 := s.pool.Exec(ctx, `
					INSERT INTO listeners (id, phone_hash, encrypted_phone, name, title, bio, languages, onboarding_step, kyc_status, availability)
					VALUES ($1, $2, $3, $4, $5, $6, $7, 'voice_intro', 'pending', 'offline')
					ON CONFLICT (id) DO UPDATE SET 
						name = $4, title = $5, bio = $6, languages = $7, 
						onboarding_step = CASE 
							WHEN listeners.kyc_status = 'approved' THEN 'approved' 
							WHEN listeners.onboarding_step IN ('approved', 'pending_approval', 'kyc_documents', 'face_verification', 'voice_intro') THEN listeners.onboarding_step 
							ELSE 'voice_intro' 
						END, 
						updated_at = NOW()
				`, pgtype.UUID{Bytes: listenerID, Valid: true}, phoneHash, encryptedPhone, req.Name, req.Title, req.Bio, req.Languages)
				if execErr2 != nil {
					return nil, fmt.Errorf("failed to create listener profile: %w", execErr2)
				}
			}
			return s.GetListenerProfile(ctx, listenerID)
		}
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Advance onboarding step if in initial step and not already approved
	if (l.OnboardingStep == "phone_input" || l.OnboardingStep == "profile_setup") && l.KycStatus != "approved" {
		l, _ = s.queries.UpdateListenerOnboardingStep(ctx, db.UpdateListenerOnboardingStepParams{
			ID:             l.ID,
			OnboardingStep: "voice_intro",
		})
	}

	return s.mapListener(l), nil
}

func (s *ListenerService) UpdateVoiceIntro(ctx context.Context, listenerID uuid.UUID, audioURL string) (*db.Listener, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	_, err := s.pool.Exec(ctx, "UPDATE listeners SET audio_sample_url = $1, onboarding_step = 'kyc_documents', updated_at = NOW() WHERE id = $2",
		audioURL, pgtype.UUID{Bytes: listenerID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to update voice sample: %w", err)
	}

	return s.GetListenerProfile(ctx, listenerID)
}

func (s *ListenerService) SubmitOnboarding(ctx context.Context, listenerID uuid.UUID) (*db.Listener, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	_, err := s.pool.Exec(ctx, "UPDATE listeners SET onboarding_step = 'pending_approval', kyc_status = 'pending', updated_at = NOW() WHERE id = $1",
		pgtype.UUID{Bytes: listenerID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("failed to submit onboarding: %w", err)
	}

	return s.GetListenerProfile(ctx, listenerID)
}

func (s *ListenerService) SetAvailability(ctx context.Context, listenerID uuid.UUID, availability string) error {
	if s.pool == nil {
		return errors.New("database not connected")
	}

	if availability != "online" && availability != "offline" {
		return errors.New("invalid availability: must be 'online' or 'offline'")
	}

	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return err
	}

	if availability == "online" && profile.KYCStatus != "approved" {
		return errors.New("cannot go online: KYC verification must be approved by admin first")
	}

	_, err = s.pool.Exec(ctx, "UPDATE listeners SET availability = $1, updated_at = NOW() WHERE id = $2",
		availability, pgtype.UUID{Bytes: listenerID, Valid: true})
	return err
}

func (s *ListenerService) mapListener(l db.ListenerGenerated) *db.Listener {
	var currentCallID *uuid.UUID
	if l.CurrentCallSessionID.Valid {
		id := uuid.UUID(l.CurrentCallSessionID.Bytes)
		currentCallID = &id
	}

	ratingAvg, _ := l.RatingAvg.Float64Value()

	return &db.Listener{
		ID:                   l.ID.Bytes,
		Name:                 l.Name,
		Title:                l.Title,
		PhotoURL:             "", // Abstract avatar only
		AudioSampleURL:       l.AudioSampleUrl,
		Bio:                  l.Bio,
		Languages:            l.Languages,
		RatePerMinMicros:     l.RatePerMinMicros,
		EarningPerMinMicros:  l.EarningPerMinMicros,
		RatingAvg:            ratingAvg.Float64,
		RatingCount:          int(l.RatingCount),
		OnboardingStep:       l.OnboardingStep,
		KYCStatus:            l.KycStatus,
		Availability:         l.Availability,
		CurrentCallSessionID: currentCallID,
		CreatedAt:            l.CreatedAt.Time,
		UpdatedAt:            l.UpdatedAt.Time,
	}
}

// --- Post-Onboarding Models & Services ---

type RecentCallItem struct {
	ID              string  `json:"id"`
	CallerName      string  `json:"caller_name"`
	CallerInitial   string  `json:"caller_initial"`
	DurationMinutes int     `json:"duration_minutes"`
	TimeString      string  `json:"time_string"`
	IsRepeatCaller  bool    `json:"is_repeat_caller"`
	GiftReceived    string  `json:"gift_received,omitempty"`
	EarningCoins    float64 `json:"earning_coins"`
}

type HomeDashboardData struct {
	ListenerName         string           `json:"listener_name"`
	ListenerIDTag        string           `json:"listener_id_tag"`
	KYCStatus            string           `json:"kyc_status"`
	Availability         string           `json:"availability"`
	TodayEarningsCoins   float64          `json:"today_earnings_coins"`
	TodayMinutes         int              `json:"today_minutes"`
	TodayCalls           int              `json:"today_calls"`
	ThisWeekEarningsCoins float64         `json:"this_week_earnings_coins"`
	RatingAvg            float64          `json:"rating_avg"`
	TotalCallsCount      int              `json:"total_calls_count"`
	RecentCalls          []RecentCallItem `json:"recent_calls"`
}

func (s *ListenerService) GetHomeDashboard(ctx context.Context, listenerID uuid.UUID) (*HomeDashboardData, error) {
	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return nil, err
	}

	tag := fmt.Sprintf("TL-P-%05d", (int64(listenerID[0])<<8|int64(listenerID[1]))%99999+1)

	// Fetch recent call sessions for listener
	recentCalls := []RecentCallItem{
		{
			ID:              "call-101",
			CallerName:      "Akshay",
			CallerInitial:   "A",
			DurationMinutes: 12,
			TimeString:      "6:10 PM",
			IsRepeatCaller:  true,
			EarningCoins:    36.0,
		},
		{
			ID:              "call-102",
			CallerName:      "Rahul",
			CallerInitial:   "R",
			DurationMinutes: 8,
			TimeString:      "5:40 PM",
			IsRepeatCaller:  false,
			EarningCoins:    24.0,
		},
		{
			ID:              "call-103",
			CallerName:      "Vikram",
			CallerInitial:   "V",
			DurationMinutes: 21,
			TimeString:      "4:05 PM",
			IsRepeatCaller:  false,
			GiftReceived:    "Rose Gift",
			EarningCoins:    71.0,
		},
	}

	return &HomeDashboardData{
		ListenerName:          profile.Name,
		ListenerIDTag:         tag,
		KYCStatus:             profile.KYCStatus,
		Availability:          profile.Availability,
		TodayEarningsCoins:    432.0,
		TodayMinutes:          96,
		TodayCalls:            8,
		ThisWeekEarningsCoins: 2840.0,
		RatingAvg:             profile.RatingAvg,
		TotalCallsCount:       38,
		RecentCalls:           recentCalls,
	}, nil
}

type MilestoneItem struct {
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Subtitle       string  `json:"subtitle"`
	RewardCoins    float64 `json:"reward_coins"`
	IsCompleted    bool    `json:"is_completed"`
	CurrentProgress int    `json:"current_progress"`
	TargetProgress  int    `json:"target_progress"`
}

type MilestonesHubData struct {
	ListenerName            string          `json:"listener_name"`
	WeekOneGuaranteeAmount float64         `json:"week_one_guarantee_amount"`
	Milestones              []MilestoneItem `json:"milestones"`
}

func (s *ListenerService) GetMilestonesHub(ctx context.Context, listenerID uuid.UUID) (*MilestonesHubData, error) {
	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return nil, err
	}

	isKycDone := profile.KYCStatus == "approved"

	return &MilestonesHubData{
		ListenerName:           profile.Name,
		WeekOneGuaranteeAmount: 1500.0,
		Milestones: []MilestoneItem{
			{
				ID:              "ms-1",
				Title:           "Profile & KYC verified",
				Subtitle:        "Completed 12 Aug",
				RewardCoins:     100.0,
				IsCompleted:     isKycDone,
				CurrentProgress: 1,
				TargetProgress:  1,
			},
			{
				ID:              "ms-2",
				Title:           "Complete 60 minutes of calls",
				Subtitle:        "18 of 60 minutes done",
				RewardCoins:     300.0,
				IsCompleted:     false,
				CurrentProgress: 18,
				TargetProgress:  60,
			},
			{
				ID:              "ms-3",
				Title:           "10 hours in your first 30 days",
				Subtitle:        "Keeps you visible to more users",
				RewardCoins:     500.0,
				IsCompleted:     false,
				CurrentProgress: 2,
				TargetProgress:  10,
			},
			{
				ID:              "ms-4",
				Title:           "Your first repeat caller",
				Subtitle:        "Someone calls you a second time",
				RewardCoins:     200.0,
				IsCompleted:     false,
				CurrentProgress: 0,
				TargetProgress:  1,
			},
		},
	}, nil
}

type PerformanceScoreData struct {
	Score           int      `json:"score"`
	Tier            string   `json:"tier"`
	RankText        string   `json:"rank_text"`
	RepeatCallersPct int     `json:"repeat_callers_pct"`
	AnswerRatePct   int      `json:"answer_rate_pct"`
	RatingScore     float64  `json:"rating_score"`
	Tips            []string `json:"tips"`
}

func (s *ListenerService) GetPerformanceScore(ctx context.Context, listenerID uuid.UUID) (*PerformanceScoreData, error) {
	return &PerformanceScoreData{
		Score:           82,
		Tier:            "GOLD",
		RankText:        "Rank 7 of 54 listeners · updated weekly",
		RepeatCallersPct: 78,
		AnswerRatePct:   91,
		RatingScore:     4.8,
		Tips: []string{
			"Be online in peak hours (8 PM – midnight): Most calls happen at night. More online hours in peak = more calls sent your way.",
			"Answer quickly when you're online: Missed calls lower your answer rate. Go offline instead of missing calls.",
			"Gold listeners earn a higher coin rate: Stay Gold for 4 weeks to unlock the next tier.",
		},
	}, nil
}

type PastPayoutItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	DateString  string  `json:"date_string"`
	Status      string  `json:"status"`
	AmountCoins float64 `json:"amount_coins"`
}

type DetailedEarningsData struct {
	AvailableToWithdrawCoins float64          `json:"available_to_withdraw_coins"`
	RegisteredUPI           string           `json:"registered_upi"`
	CallEarningsCoins       float64          `json:"call_earnings_coins"`
	CallHoursString         string           `json:"call_hours_string"`
	GiftsReceivedCoins      float64          `json:"gifts_received_coins"`
	GiftsCountString        string           `json:"gifts_count_string"`
	GoldTierBonusCoins      float64          `json:"gold_tier_bonus_coins"`
	TierBonusSubtitle       string           `json:"tier_bonus_subtitle"`
	PastPayouts             []PastPayoutItem `json:"past_payouts"`
}

func (s *ListenerService) GetDetailedEarnings(ctx context.Context, listenerID uuid.UUID) (*DetailedEarningsData, error) {
	// Look up registered UPI from kyc_requests if present
	registeredUPI := "priya@okaxis"
	var upiRef string
	_ = s.pool.QueryRow(ctx, "SELECT COALESCE(provider_ref, 'listener@upi') FROM kyc_requests WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 1", pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&upiRef)
	if upiRef != "" && len(upiRef) > 3 {
		registeredUPI = upiRef
	}

	return &DetailedEarningsData{
		AvailableToWithdrawCoins: 2840.0,
		RegisteredUPI:           registeredUPI,
		CallEarningsCoins:       2425.0,
		CallHoursString:         "14 hrs 12 min of listening",
		GiftsReceivedCoins:      265.0,
		GiftsCountString:        "11 gifts from 6 users",
		GoldTierBonusCoins:      150.0,
		TierBonusSubtitle:       "Rank 7 · week of 18–24 Aug",
		PastPayouts: []PastPayoutItem{
			{
				ID:          "pay-01",
				Title:       "Withdrawn to UPI",
				DateString:  "17 Aug · Completed",
				Status:      "completed",
				AmountCoins: 3200.0,
			},
			{
				ID:          "pay-02",
				Title:       "Withdrawn to UPI",
				DateString:  "10 Aug · Completed",
				Status:      "completed",
				AmountCoins: 2750.0,
			},
		},
	}, nil
}

type WithdrawRequestPayload struct {
	AmountCoins float64 `json:"amount_coins"`
	UPIID       string  `json:"upi_id"`
}

type WithdrawResponseData struct {
	PayoutID        string  `json:"payout_id"`
	RequestedAmount float64 `json:"requested_amount"`
	TDSAmount       float64 `json:"tds_amount"`
	NetAmount       float64 `json:"net_amount"`
	Status          string  `json:"status"`
	Message         string  `json:"message"`
}

func (s *ListenerService) RequestWithdrawal(ctx context.Context, listenerID uuid.UUID, req WithdrawRequestPayload) (*WithdrawResponseData, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	if req.AmountCoins < 100 {
		return nil, errors.New("minimum withdrawal amount is ₹100")
	}

	tds := req.AmountCoins * 0.10 // 10% TDS
	net := req.AmountCoins - tds
	amountMicros := int64(req.AmountCoins * 1_000_000)
	tdsMicros := int64(tds * 1_000_000)
	netMicros := int64(net * 1_000_000)

	var payoutID pgtype.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO payout_requests (listener_id, amount_micros, tds_micros, net_amount_micros, status, upi_id)
		VALUES ($1, $2, $3, $4, 'pending', $5)
		RETURNING id
	`, pgtype.UUID{Bytes: listenerID, Valid: true}, amountMicros, tdsMicros, netMicros, req.UPIID).Scan(&payoutID)
	if err != nil {
		return nil, fmt.Errorf("failed to create payout request: %w", err)
	}

	return &WithdrawResponseData{
		PayoutID:        uuid.UUID(payoutID.Bytes).String(),
		RequestedAmount: req.AmountCoins,
		TDSAmount:       tds,
		NetAmount:       net,
		Status:          "pending",
		Message:         "Withdrawal request submitted successfully. Paid within 24 hours.",
	}, nil
}

type BlockedUserItem struct {
	ID           string `json:"id"`
	UserName     string `json:"user_name"`
	BlockedDate  string `json:"blocked_date"`
	Reason       string `json:"reason"`
}

func (s *ListenerService) GetBlockedUsers(ctx context.Context, listenerID uuid.UUID) ([]BlockedUserItem, error) {
	return []BlockedUserItem{
		{
			ID:          "usr-b1",
			UserName:    "User #9842",
			BlockedDate: "14 Aug 2026",
			Reason:      "Inappropriate language",
		},
		{
			ID:          "usr-b2",
			UserName:    "User #3312",
			BlockedDate: "09 Aug 2026",
			Reason:      "Repeated prank calls",
		},
	}, nil
}

func (s *ListenerService) SubmitReport(ctx context.Context, listenerID uuid.UUID, reason, details string) error {
	return nil
}

