package listeners

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

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

var defaultListenerNames = []string{
	"Ahana", "Aira", "Alia", "Aliza", "Amaira", "Amaya", "Anvi", "Arzoi", "Avani", "Aviana",
	"Avni", "Ayat", "Barkha", "Chahat", "Charvi", "Daksha", "Disha", "Drishti", "Driti", "Elina",
	"Eva", "Evanya", "Gazal", "Greeshma", "Hiya", "Iba", "Ilisha", "Inara", "Inaya", "Ira",
	"Ivana", "Jannat", "Jasnoor", "Jiana", "Jiya", "Kainaat", "Kashish", "Khushi", "Kiara", "Kimaya",
	"Kisha", "Lavanya", "Liana", "Liya", "Mahika", "Mayra", "Meher", "Mihika", "Miraya", "Misha",
	"Mishka", "Myra", "Navya", "Nayonika", "Nehal", "Nia", "Niharika", "Nikita", "Nisa", "Nisha",
	"Noya", "Nyra", "Pahel", "Pakhi", "Palak", "Pari", "Parina", "Purva", "Rhea", "Ria",
	"Rida", "Rimi", "Risha", "Riti", "Riya", "Roshni", "Ruhi", "Saira", "Samaira", "Sanaya",
	"Sara", "Seher", "Shanaya", "Shina", "Simra", "Siya", "Suhana", "Suhani", "Taara", "Tanisha",
	"Tanya", "Tara", "Tisha", "Trisha", "Vamika", "Vanya", "Zara", "Zayan", "Zoya", "Zuha",
}

func getAutoAssignedListenerName(id uuid.UUID) string {
	idx := int(id[0]^id[1]) % len(defaultListenerNames)
	if idx < 0 {
		idx = -idx
	}
	return defaultListenerNames[idx]
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

	res := s.mapListener(l)
	if res.Name == "" || strings.EqualFold(res.Name, "Listener") {
		autoName := getAutoAssignedListenerName(listenerID)
		res.Name = autoName
		_, _ = s.pool.Exec(ctx, "UPDATE listeners SET name = $1, updated_at = NOW() WHERE id = $2 AND (name = '' OR name = 'Listener')", autoName, pgtype.UUID{Bytes: listenerID, Valid: true})
	}

	return res, nil
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

	if availability != "online" && availability != "offline" && availability != "busy" {
		return errors.New("invalid availability: must be 'online', 'busy', or 'offline'")
	}

	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return err
	}

	if (availability == "online" || availability == "busy") && profile.KYCStatus != "approved" {
		return errors.New("cannot go online: KYC verification must be approved by admin first")
	}

	_, err = s.pool.Exec(ctx, "UPDATE listeners SET availability = $1, updated_at = NOW() WHERE id = $2",
		availability, pgtype.UUID{Bytes: listenerID, Valid: true})
	if err != nil {
		return err
	}

	if availability == "online" {
		// Mark pending waitlist users as notified
		_, _ = s.pool.Exec(ctx, "UPDATE listener_waitlist SET notified = TRUE WHERE listener_id = $1 AND notified = FALSE", pgtype.UUID{Bytes: listenerID, Valid: true})
	}

	return nil
}

func (s *ListenerService) SubscribeNotifyWhenOnline(ctx context.Context, userID, listenerID uuid.UUID) error {
	if s.pool == nil {
		return errors.New("database not connected")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO listener_waitlist (user_id, listener_id, notified, created_at)
		VALUES ($1, $2, FALSE, NOW())
	`, pgtype.UUID{Bytes: userID, Valid: true}, pgtype.UUID{Bytes: listenerID, Valid: true})
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
	ListenerName          string           `json:"listener_name"`
	ListenerIDTag         string           `json:"listener_id_tag"`
	KYCStatus             string           `json:"kyc_status"`
	Availability          string           `json:"availability"`
	TodayEarningsCoins    float64          `json:"today_earnings_coins"`
	TodayMinutes          int              `json:"today_minutes"`
	TodayCalls            int              `json:"today_calls"`
	ThisWeekEarningsCoins float64          `json:"this_week_earnings_coins"`
	RatingAvg             float64          `json:"rating_avg"`
	RatingCount           int              `json:"rating_count"`
	AnswerRatePct         int              `json:"answer_rate_pct"`
	TotalCallsCount       int              `json:"total_calls_count"`
	RecentCalls           []RecentCallItem `json:"recent_calls"`
}

func (s *ListenerService) GetHomeDashboard(ctx context.Context, listenerID uuid.UUID) (*HomeDashboardData, error) {
	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return nil, err
	}

	tag := fmt.Sprintf("TL-P-%05d", (int64(listenerID[0])<<8|int64(listenerID[1]))%99999+1)

	var totalCallsCount, todayCalls, todayMinutes int
	var todayEarningsMicros, thisWeekEarningsMicros int64
	var answeredCallsCount, totalIncomingCount int

	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'ended'),
			COUNT(*) FILTER (WHERE status = 'ended' AND created_at >= CURRENT_DATE),
			COALESCE(SUM(duration_seconds) FILTER (WHERE status = 'ended' AND created_at >= CURRENT_DATE), 0) / 60,
			COALESCE(SUM(listener_earning_micros) FILTER (WHERE status = 'ended' AND created_at >= CURRENT_DATE), 0),
			COALESCE(SUM(listener_earning_micros) FILTER (WHERE status = 'ended' AND created_at >= date_trunc('week', CURRENT_DATE)), 0),
			COUNT(*) FILTER (WHERE status IN ('ended', 'accepted')),
			COUNT(*)
		FROM call_sessions
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(
		&totalCallsCount,
		&todayCalls,
		&todayMinutes,
		&todayEarningsMicros,
		&thisWeekEarningsMicros,
		&answeredCallsCount,
		&totalIncomingCount,
	)

	var ratingAvg float64
	var ratingCount int
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(stars)::numeric, 1), 0.0), COUNT(*)
		FROM (
			SELECT stars FROM ratings WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 50
		) r
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&ratingAvg, &ratingCount)

	answerRatePct := 0
	if totalIncomingCount > 0 {
		answerRatePct = int(float64(answeredCallsCount) / float64(totalIncomingCount) * 100.0)
	}

	recentCalls := make([]RecentCallItem, 0)
	rows, err := s.pool.Query(ctx, `
		SELECT cs.id, COALESCE(u.display_name, u.name, 'User'), cs.duration_seconds, cs.created_at, cs.listener_earning_micros
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		WHERE cs.listener_id = $1 AND cs.status = 'ended'
		ORDER BY cs.created_at DESC
		LIMIT 10
	`, pgtype.UUID{Bytes: listenerID, Valid: true})
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var callID, callerName string
			var durSec int
			var createdAt time.Time
			var earnMicros int64
			if err := rows.Scan(&callID, &callerName, &durSec, &createdAt, &earnMicros); err == nil {
				initial := "U"
				if len(callerName) > 0 {
					initial = string([]rune(callerName)[0])
				}
				recentCalls = append(recentCalls, RecentCallItem{
					ID:              callID,
					CallerName:      callerName,
					CallerInitial:   strings.ToUpper(initial),
					DurationMinutes: durSec / 60,
					TimeString:      createdAt.Format("3:04 PM"),
					IsRepeatCaller:  false,
					EarningCoins:    float64(earnMicros) / 1000000.0,
				})
			}
		}
	}

	return &HomeDashboardData{
		ListenerName:          profile.Name,
		ListenerIDTag:         tag,
		KYCStatus:             profile.KYCStatus,
		Availability:          profile.Availability,
		TodayEarningsCoins:    float64(todayEarningsMicros) / 1000000.0,
		TodayMinutes:          todayMinutes,
		TodayCalls:            todayCalls,
		ThisWeekEarningsCoins: float64(thisWeekEarningsMicros) / 1000000.0,
		RatingAvg:             ratingAvg,
		RatingCount:           ratingCount,
		AnswerRatePct:         answerRatePct,
		TotalCallsCount:       totalCallsCount,
		RecentCalls:           recentCalls,
	}, nil
}

type MilestoneItem struct {
	ID              string  `json:"id"`
	Title           string  `json:"title"`
	Subtitle        string  `json:"subtitle"`
	RewardCoins     float64 `json:"reward_coins"`
	IsCompleted     bool    `json:"is_completed"`
	CurrentProgress int     `json:"current_progress"`
	TargetProgress  int     `json:"target_progress"`
}

type MilestonesHubData struct {
	ListenerName           string          `json:"listener_name"`
	WeekOneGuaranteeAmount float64         `json:"week_one_guarantee_amount"`
	Milestones             []MilestoneItem `json:"milestones"`
}

func (s *ListenerService) GetMilestonesHub(ctx context.Context, listenerID uuid.UUID) (*MilestonesHubData, error) {
	profile, err := s.GetListenerProfile(ctx, listenerID)
	if err != nil {
		return nil, err
	}

	isKycDone := profile.KYCStatus == "approved"

	var totalMinutes int
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(duration_seconds) FILTER (WHERE status = 'ended'), 0) / 60
		FROM call_sessions
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&totalMinutes)

	return &MilestonesHubData{
		ListenerName:           profile.Name,
		WeekOneGuaranteeAmount: 1500.0,
		Milestones: []MilestoneItem{
			{
				ID:              "ms-1",
				Title:           "Profile & KYC verified",
				Subtitle:        "Complete identity verification",
				RewardCoins:     100.0,
				IsCompleted:     isKycDone,
				CurrentProgress: 1,
				TargetProgress:  1,
			},
			{
				ID:              "ms-2",
				Title:           "Complete 60 minutes of calls",
				Subtitle:        fmt.Sprintf("%d of 60 minutes done", totalMinutes),
				RewardCoins:     300.0,
				IsCompleted:     totalMinutes >= 60,
				CurrentProgress: totalMinutes,
				TargetProgress:  60,
			},
			{
				ID:              "ms-3",
				Title:           "10 hours in your first 30 days",
				Subtitle:        fmt.Sprintf("%d of 10 hours completed", totalMinutes/60),
				RewardCoins:     500.0,
				IsCompleted:     totalMinutes >= 600,
				CurrentProgress: totalMinutes / 60,
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
	Score            int      `json:"score"`
	Tier             string   `json:"tier"`
	RankText         string   `json:"rank_text"`
	RepeatCallersPct int      `json:"repeat_callers_pct"`
	AnswerRatePct    int      `json:"answer_rate_pct"`
	RatingScore      float64  `json:"rating_score"`
	Tips             []string `json:"tips"`
}

func (s *ListenerService) GetPerformanceScore(ctx context.Context, listenerID uuid.UUID) (*PerformanceScoreData, error) {
	var totalCallsCount, answeredCallsCount, totalIncomingCount int
	_ = s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'ended'),
			COUNT(*) FILTER (WHERE status IN ('ended', 'accepted')),
			COUNT(*)
		FROM call_sessions
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&totalCallsCount, &answeredCallsCount, &totalIncomingCount)

	var ratingAvg float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(stars)::numeric, 1), 0.0)
		FROM (
			SELECT stars FROM ratings WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 50
		) r
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&ratingAvg)

	answerRatePct := 0
	if totalIncomingCount > 0 {
		answerRatePct = int(float64(answeredCallsCount) / float64(totalIncomingCount) * 100.0)
	}

	tier := "BRONZE"
	score := 0
	if totalCallsCount > 50 {
		tier = "GOLD"
		score = 80 + int(ratingAvg*4)
	} else if totalCallsCount > 10 {
		tier = "SILVER"
		score = 50 + totalCallsCount
	} else if totalCallsCount > 0 {
		score = totalCallsCount * 5
	}

	return &PerformanceScoreData{
		Score:            score,
		Tier:             tier,
		RankText:         "Updated weekly based on active call hours and rating",
		RepeatCallersPct: 0,
		AnswerRatePct:    answerRatePct,
		RatingScore:      ratingAvg,
		Tips: []string{
			"Be online in peak hours (8 PM – midnight): Most calls happen at night. More online hours in peak = more calls sent your way.",
			"Answer quickly when you're online: Missed calls lower your answer rate. Go offline instead of missing calls.",
			"Listeners with high ratings earn more: Complete more calls to increase your rating and tier.",
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
	RegisteredUPI            string           `json:"registered_upi"`
	CallEarningsCoins        float64          `json:"call_earnings_coins"`
	CallHoursString          string           `json:"call_hours_string"`
	GiftsReceivedCoins       float64          `json:"gifts_received_coins"`
	GiftsCountString         string           `json:"gifts_count_string"`
	GoldTierBonusCoins       float64          `json:"gold_tier_bonus_coins"`
	TierBonusSubtitle        string           `json:"tier_bonus_subtitle"`
	PastPayouts              []PastPayoutItem `json:"past_payouts"`
}

func (s *ListenerService) GetDetailedEarnings(ctx context.Context, listenerID uuid.UUID) (*DetailedEarningsData, error) {
	var callEarningsMicros int64
	var totalSeconds int
	_ = s.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(listener_earning_micros) FILTER (WHERE status = 'ended'), 0),
			COALESCE(SUM(duration_seconds) FILTER (WHERE status = 'ended'), 0)
		FROM call_sessions
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&callEarningsMicros, &totalSeconds)

	callCoins := float64(callEarningsMicros) / 1000000.0
	hours := totalSeconds / 3600
	mins := (totalSeconds % 3600) / 60
	callHoursStr := fmt.Sprintf("%d hrs %d min of listening", hours, mins)

	var registeredUPI string
	_ = s.pool.QueryRow(ctx, "SELECT COALESCE(provider_ref, '') FROM kyc_requests WHERE listener_id = $1 ORDER BY created_at DESC LIMIT 1", pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&registeredUPI)
	if registeredUPI == "" {
		registeredUPI = "Not registered yet"
	}

	return &DetailedEarningsData{
		AvailableToWithdrawCoins: callCoins,
		RegisteredUPI:            registeredUPI,
		CallEarningsCoins:        callCoins,
		CallHoursString:          callHoursStr,
		GiftsReceivedCoins:       0.0,
		GiftsCountString:         "0 gifts received",
		GoldTierBonusCoins:       0.0,
		TierBonusSubtitle:        "Tier bonus",
		PastPayouts:              []PastPayoutItem{},
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

type CallLogHistoryItem struct {
	ID               string `json:"id"`
	AvatarText       string `json:"avatar_text"`
	CallerName       string `json:"caller_name"`
	IsMissed         bool   `json:"is_missed"`
	TimestampDetails string `json:"timestamp_details"`
	AmountStr        string `json:"amount_str"`
	IsNegative       bool   `json:"is_negative"`
	IsPeachAvatar    bool   `json:"is_peach_avatar"`
	Section          string `json:"section"` // "TODAY", "YESTERDAY", "EARLIER"
	CreatedAt        string `json:"created_at"`
}

type CallHistoryResponse struct {
	TotalAnswered  int                  `json:"total_answered"`
	AvgDurationMin float64              `json:"avg_duration_min"`
	AvgRating      float64              `json:"avg_rating"`
	RatingCount    int                  `json:"rating_count"`
	Calls          []CallLogHistoryItem `json:"calls"`
}

func (s *ListenerService) GetCallHistory(ctx context.Context, listenerID uuid.UUID) (*CallHistoryResponse, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	var totalAnswered int
	var avgDurationMin float64
	var avgRating float64
	var ratingCount int

	// 1. Stats Query (answered calls count & avg talk time)
	_ = s.pool.QueryRow(ctx, `
		SELECT 
			COUNT(*) FILTER (WHERE status = 'ended' OR started_at IS NOT NULL),
			COALESCE(ROUND(AVG(
				CASE 
					WHEN ended_at IS NOT NULL AND started_at IS NOT NULL THEN EXTRACT(EPOCH FROM (ended_at - started_at)) / 60.0
					ELSE 0 
				END
			) FILTER (WHERE status = 'ended' OR started_at IS NOT NULL)::numeric, 1), 0.0)
		FROM call_sessions
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&totalAnswered, &avgDurationMin)

	// 2. Ratings Query
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(ROUND(AVG(stars)::numeric, 1), 0.0), COUNT(*)
		FROM ratings
		WHERE listener_id = $1
	`, pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&avgRating, &ratingCount)

	// 3. Calls list query
	rows, err := s.pool.Query(ctx, `
		SELECT 
			cs.id,
			COALESCE(NULLIF(u.name, ''), 'user' || (100000 + (abs(hashtext(u.id::text)) % 900000))::text) as caller_name,
			COALESCE(u.language_pref, 'hi') as caller_lang,
			cs.status,
			cs.started_at,
			cs.ended_at,
			cs.earning_per_min_micros_snapshot,
			r.stars,
			cs.created_at
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		LEFT JOIN ratings r ON r.call_session_id = cs.id
		WHERE cs.listener_id = $1
		ORDER BY cs.created_at DESC
		LIMIT 50
	`, pgtype.UUID{Bytes: listenerID, Valid: true})

	calls := make([]CallLogHistoryItem, 0)
	if err == nil {
		defer rows.Close()
		now := time.Now()
		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		yesterdayStart := todayStart.AddDate(0, 0, -1)

		for rows.Next() {
			var id, callerName, callerLang, status string
			var startedAt, endedAt *time.Time
			var earnPerMinMicros int64
			var stars *int
			var createdAt time.Time

			if err := rows.Scan(&id, &callerName, &callerLang, &status, &startedAt, &endedAt, &earnPerMinMicros, &stars, &createdAt); err == nil {
				isMissed := status == "cancelled" || (startedAt == nil && status != "active")

				section := "EARLIER"
				if createdAt.After(todayStart) {
					section = "TODAY"
				} else if createdAt.After(yesterdayStart) {
					section = "YESTERDAY"
				}

				timeStr := createdAt.Format("3:04 PM")
				var details string
				var amountStr string
				var isNegative bool
				var isPeachAvatar bool

				langName := "Hindi"
				switch strings.ToLower(callerLang) {
				case "en":
					langName = "English"
				case "bn":
					langName = "Bengali"
				case "ta":
					langName = "Tamil"
				case "te":
					langName = "Telugu"
				case "mr":
					langName = "Marathi"
				}

				displayName := callerName + " · " + langName
				if isMissed {
					displayName = callerName + " · missed"
					details = timeStr + " · missed"
					amountStr = "₹0"
					isNegative = false
					isPeachAvatar = true
				} else {
					var durSec int
					if startedAt != nil && endedAt != nil {
						durSec = int(endedAt.Sub(*startedAt).Seconds())
					}
					durM := durSec / 60
					durS := durSec % 60

					details = fmt.Sprintf("%s · %d min %02d s", timeStr, durM, durS)
					if stars != nil && *stars > 0 {
						details += fmt.Sprintf(" · ★ %d", *stars)
					}
					earnCoins := (float64(earnPerMinMicros) / 1000000.0) * (float64(durSec) / 60.0)
					amountStr = fmt.Sprintf("₹%.2f", earnCoins)
				}

				avatarText := "US"
				if len(callerName) >= 2 {
					avatarText = strings.ToUpper(callerName[:2])
				}

				calls = append(calls, CallLogHistoryItem{
					ID:               id,
					AvatarText:       avatarText,
					CallerName:       displayName,
					IsMissed:         isMissed,
					TimestampDetails: details,
					AmountStr:        amountStr,
					IsNegative:       isNegative,
					IsPeachAvatar:    isPeachAvatar,
					Section:          section,
					CreatedAt:        createdAt.Format(time.RFC3339),
				})
			}
		}
	}

	return &CallHistoryResponse{
		TotalAnswered:  totalAnswered,
		AvgDurationMin: avgDurationMin,
		AvgRating:      avgRating,
		RatingCount:    ratingCount,
		Calls:          calls,
	}, nil
}

type TransactionItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Timestamp   string `json:"timestamp"`
	Amount      string `json:"amount"`
	Status      string `json:"status"`
	StatusColor string `json:"status_color"`
	IsPositive  bool   `json:"is_positive"`
	FilterType  string `json:"filter_type"` // "CALLS", "BONUS", "PAYOUT", "PENALTY"
	MonthGroup  string `json:"month_group"` // "AUG 2026"
	CreatedAt   string `json:"created_at"`
}

type TransactionsResponse struct {
	Transactions []TransactionItem `json:"transactions"`
}

func (s *ListenerService) GetTransactions(ctx context.Context, listenerID uuid.UUID) (*TransactionsResponse, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	transactions := make([]TransactionItem, 0)

	// 1. Fetch Call Transactions
	rows, err := s.pool.Query(ctx, `
		SELECT 
			cs.id,
			COALESCE(NULLIF(u.name, ''), 'caller ' || SUBSTRING(REPLACE(cs.user_id::text, '-', ''), 1, 4)) as caller_name,
			cs.status,
			cs.started_at,
			cs.ended_at,
			cs.earning_per_min_micros_snapshot,
			cs.created_at
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		WHERE cs.listener_id = $1 AND cs.status = 'ended' AND cs.started_at IS NOT NULL AND cs.ended_at IS NOT NULL
		ORDER BY cs.created_at DESC
		LIMIT 50
	`, pgtype.UUID{Bytes: listenerID, Valid: true})

	if err == nil {
		defer rows.Close()
		now := time.Now()
		for rows.Next() {
			var id, callerName, status string
			var startedAt, endedAt *time.Time
			var earnPerMinMicros int64
			var createdAt time.Time

			if err := rows.Scan(&id, &callerName, &status, &startedAt, &endedAt, &earnPerMinMicros, &createdAt); err == nil {
				durSec := 0
				if startedAt != nil && endedAt != nil {
					durSec = int(endedAt.Sub(*startedAt).Seconds())
				}
				durM := durSec / 60
				durS := durSec % 60
				durFormatted := fmt.Sprintf("%02d:%02d", durM, durS)

				timeAndDate := createdAt.Format("02 Jan, 3:04 PM")
				timestampStr := fmt.Sprintf("%s · %s", durFormatted, timeAndDate)

				earnCoins := (float64(earnPerMinMicros) / 1000000.0) * (float64(durSec) / 60.0)
				amountStr := fmt.Sprintf("+ ₹%.2f", earnCoins)

				txStatus := "Pending"
				statusColor := "orange"
				if now.Sub(createdAt) > 48*time.Hour {
					txStatus = "Cleared"
					statusColor = "gray"
				}

				monthGroup := strings.ToUpper(createdAt.Format("Jan 2006"))

				transactions = append(transactions, TransactionItem{
					ID:          id,
					Title:       "Call · " + callerName,
					Timestamp:   timestampStr,
					Amount:      amountStr,
					Status:      txStatus,
					StatusColor: statusColor,
					IsPositive:  true,
					FilterType:  "CALLS",
					MonthGroup:  monthGroup,
					CreatedAt:   createdAt.Format(time.RFC3339),
				})
			}
		}
	}

	// 2. Fetch Weekly Volume Target Bonuses (500+ total minutes across ALL users in a week)
	bonusRows, bErr := s.pool.Query(ctx, `
		SELECT 
			date_trunc('week', cs.created_at) as week_start,
			COALESCE(SUM(EXTRACT(EPOCH FROM (cs.ended_at - cs.started_at))) / 60, 0) as total_min,
			MAX(cs.created_at) as last_call_time
		FROM call_sessions cs
		WHERE cs.listener_id = $1 
		  AND cs.status = 'ended' 
		  AND cs.started_at IS NOT NULL 
		  AND cs.ended_at IS NOT NULL
		GROUP BY date_trunc('week', cs.created_at)
		HAVING COALESCE(SUM(EXTRACT(EPOCH FROM (cs.ended_at - cs.started_at))) / 60, 0) >= 500
		ORDER BY week_start DESC
	`, pgtype.UUID{Bytes: listenerID, Valid: true})

	if bErr == nil {
		defer bonusRows.Close()
		now := time.Now()
		for bonusRows.Next() {
			var weekStart, lastCallTime time.Time
			var totalMin int64
			if err := bonusRows.Scan(&weekStart, &totalMin, &lastCallTime); err == nil {
				weekEnd := weekStart.AddDate(0, 0, 6)
				dateStr := fmt.Sprintf("%s – %s", weekStart.Format("02 Jan"), weekEnd.Format("02 Jan, 2006"))
				monthGroup := strings.ToUpper(lastCallTime.Format("Jan 2006"))

				txStatus := "Cleared"
				statusColor := "gray"
				if now.Before(weekEnd.AddDate(0, 0, 1)) {
					txStatus = "Pending"
					statusColor = "orange"
				}

				transactions = append(transactions, TransactionItem{
					ID:          fmt.Sprintf("bonus_%s", weekStart.Format("20060102")),
					Title:       fmt.Sprintf("Weekly Volume Bonus · %d min", totalMin),
					Timestamp:   dateStr,
					Amount:      "+ ₹150.00",
					Status:      txStatus,
					StatusColor: statusColor,
					IsPositive:  true,
					FilterType:  "BONUS",
					MonthGroup:  monthGroup,
					CreatedAt:   lastCallTime.Format(time.RFC3339),
				})
			}
		}
	}

	// 3. Fetch Payout Transactions
	payoutRows, pErr := s.pool.Query(ctx, `
		SELECT 
			id,
			net_amount_micros,
			status,
			upi_id,
			requested_at
		FROM payout_requests
		WHERE listener_id = $1
		ORDER BY requested_at DESC
		LIMIT 20
	`, pgtype.UUID{Bytes: listenerID, Valid: true})

	if pErr == nil {
		defer payoutRows.Close()
		for payoutRows.Next() {
			var id string
			var netAmountMicros int64
			var status, upiID string
			var requestedAt time.Time

			if err := payoutRows.Scan(&id, &netAmountMicros, &status, &upiID, &requestedAt); err == nil {
				maskedUPI := "••••"
				if len(upiID) > 4 {
					maskedUPI += upiID[len(upiID)-4:]
				}

				coins := float64(netAmountMicros) / 1000000.0
				amountStr := fmt.Sprintf("₹%.2f", coins)

				txStatus := "Pending"
				if status == "paid" {
					txStatus = "Paid"
				}

				monthGroup := strings.ToUpper(requestedAt.Format("Jan 2006"))
				timestampStr := requestedAt.Format("02 Jan, 3:04 PM")

				transactions = append(transactions, TransactionItem{
					ID:          id,
					Title:       "Payout to UPI " + maskedUPI,
					Timestamp:   timestampStr,
					Amount:      amountStr,
					Status:      txStatus,
					StatusColor: "gray",
					IsPositive:  false,
					FilterType:  "PAYOUT",
					MonthGroup:  monthGroup,
					CreatedAt:   requestedAt.Format(time.RFC3339),
				})
			}
		}
	}

	return &TransactionsResponse{
		Transactions: transactions,
	}, nil
}

type NotificationItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"`       // "PAYOUT", "BONUS", "RATING", "MISSED_CALL", "KYC"
	Title     string `json:"title"`      // e.g. "5-Star Rating Received", "Weekly Payout Dispatched"
	Message   string `json:"message"`    // e.g. "A caller gave you 5 stars!"
	Timestamp string `json:"timestamp"`  // e.g. "Today, 2:15 PM"
	IsRead    bool   `json:"is_read"`
	IconType  string `json:"icon_type"`  // "star", "wallet", "missed_call", "bonus", "shield"
	CreatedAt string `json:"created_at"`
}

type NotificationsResponse struct {
	UnreadCount   int                `json:"unread_count"`
	Notifications []NotificationItem `json:"notifications"`
}

func formatNotificationTimestamp(t time.Time) string {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	yesterdayStart := todayStart.AddDate(0, 0, -1)

	if t.After(todayStart) {
		return fmt.Sprintf("Today, %s", t.Format("3:04 PM"))
	} else if t.After(yesterdayStart) {
		return fmt.Sprintf("Yesterday, %s", t.Format("3:04 PM"))
	}
	return t.Format("02 Jan, 3:04 PM")
}

func (s *ListenerService) GetNotifications(ctx context.Context, listenerID uuid.UUID) (*NotificationsResponse, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	notifications := make([]NotificationItem, 0)
	now := time.Now()

	// 1. Real Ratings Received
	ratingRows, rErr := s.pool.Query(ctx, `
		SELECT r.id, r.stars, COALESCE(r.review_text, ''), r.created_at,
		       COALESCE(NULLIF(u.name, ''), 'A caller') as caller_name
		FROM ratings r
		LEFT JOIN users u ON u.id = r.user_id
		WHERE r.listener_id = $1
		ORDER BY r.created_at DESC
		LIMIT 20
	`, pgtype.UUID{Bytes: listenerID, Valid: true})
	if rErr == nil {
		defer ratingRows.Close()
		for ratingRows.Next() {
			var id, reviewText, callerName string
			var stars int
			var createdAt time.Time
			if err := ratingRows.Scan(&id, &stars, &reviewText, &createdAt, &callerName); err == nil {
				msg := fmt.Sprintf("%s gave you a %d-star rating for your recent call.", callerName, stars)
				if reviewText != "" {
					msg = fmt.Sprintf("%s gave you %d stars: \"%s\"", callerName, stars, reviewText)
				}
				notifications = append(notifications, NotificationItem{
					ID:        "rating_" + id,
					Type:      "RATING",
					Title:     fmt.Sprintf("%d-Star Rating Received", stars),
					Message:   msg,
					Timestamp: formatNotificationTimestamp(createdAt),
					IsRead:    now.Sub(createdAt) > 24*time.Hour,
					IconType:  "star",
					CreatedAt: createdAt.Format(time.RFC3339),
				})
			}
		}
	}

	// 2. Real Missed Calls
	missedRows, mErr := s.pool.Query(ctx, `
		SELECT cs.id, 
		       COALESCE(NULLIF(u.name, ''), 'caller ' || SUBSTRING(REPLACE(cs.user_id::text, '-', ''), 1, 4)) as caller_name,
		       cs.created_at
		FROM call_sessions cs
		LEFT JOIN users u ON u.id = cs.user_id
		WHERE cs.listener_id = $1 AND (cs.status = 'cancelled' OR (cs.status = 'ended' AND cs.started_at IS NULL))
		ORDER BY cs.created_at DESC
		LIMIT 15
	`, pgtype.UUID{Bytes: listenerID, Valid: true})
	if mErr == nil {
		defer missedRows.Close()
		for missedRows.Next() {
			var id, callerName string
			var createdAt time.Time
			if err := missedRows.Scan(&id, &callerName, &createdAt); err == nil {
				notifications = append(notifications, NotificationItem{
					ID:        "missed_" + id,
					Type:      "MISSED_CALL",
					Title:     "Missed Call Alert",
					Message:   fmt.Sprintf("You missed an incoming call from %s while online. Keep your volume up to not miss earnings!", callerName),
					Timestamp: formatNotificationTimestamp(createdAt),
					IsRead:    now.Sub(createdAt) > 24*time.Hour,
					IconType:  "missed_call",
					CreatedAt: createdAt.Format(time.RFC3339),
				})
			}
		}
	}

	// 3. Real Payout Requests & Transfers
	payoutRows, pErr := s.pool.Query(ctx, `
		SELECT id, net_amount_micros, status, upi_id, requested_at, COALESCE(processed_at, requested_at)
		FROM payout_requests
		WHERE listener_id = $1
		ORDER BY requested_at DESC
		LIMIT 15
	`, pgtype.UUID{Bytes: listenerID, Valid: true})
	if pErr == nil {
		defer payoutRows.Close()
		for payoutRows.Next() {
			var id, status, upiID string
			var netMicros int64
			var reqAt, procAt time.Time
			if err := payoutRows.Scan(&id, &netMicros, &status, &upiID, &reqAt, &procAt); err == nil {
				maskedUPI := "••••"
				if len(upiID) > 4 {
					maskedUPI += upiID[len(upiID)-4:]
				}
				amountCoins := float64(netMicros) / 1e6

				if status == "paid" {
					notifications = append(notifications, NotificationItem{
						ID:        "payout_" + id,
						Type:      "PAYOUT",
						Title:     "Weekly Payout Dispatched",
						Message:   fmt.Sprintf("₹%.2f has been successfully transferred to your UPI (%s).", amountCoins, maskedUPI),
						Timestamp: formatNotificationTimestamp(procAt),
						IsRead:    now.Sub(procAt) > 24*time.Hour,
						IconType:  "wallet",
						CreatedAt: procAt.Format(time.RFC3339),
					})
				} else {
					notifications = append(notifications, NotificationItem{
						ID:        "payout_" + id,
						Type:      "PAYOUT",
						Title:     "Payout Request Submitted",
						Message:   fmt.Sprintf("Your payout request for ₹%.2f to UPI (%s) is processing and will clear within 24 hours.", amountCoins, maskedUPI),
						Timestamp: formatNotificationTimestamp(reqAt),
						IsRead:    now.Sub(reqAt) > 24*time.Hour,
						IconType:  "wallet",
						CreatedAt: reqAt.Format(time.RFC3339),
					})
				}
			}
		}
	}

	// 4. Real Weekly Volume Bonus (500+ min across all callers)
	bonusRows, bErr := s.pool.Query(ctx, `
		SELECT date_trunc('week', cs.created_at) as week_start,
		       COALESCE(SUM(EXTRACT(EPOCH FROM (cs.ended_at - cs.started_at))) / 60, 0) as total_min,
		       MAX(cs.created_at) as last_call_time
		FROM call_sessions cs
		WHERE cs.listener_id = $1 AND cs.status = 'ended' AND cs.started_at IS NOT NULL AND cs.ended_at IS NOT NULL
		GROUP BY date_trunc('week', cs.created_at)
		HAVING COALESCE(SUM(EXTRACT(EPOCH FROM (cs.ended_at - cs.started_at))) / 60, 0) >= 500
		ORDER BY week_start DESC
		LIMIT 10
	`, pgtype.UUID{Bytes: listenerID, Valid: true})
	if bErr == nil {
		defer bonusRows.Close()
		for bonusRows.Next() {
			var weekStart, lastCallTime time.Time
			var totalMin int64
			if err := bonusRows.Scan(&weekStart, &totalMin, &lastCallTime); err == nil {
				notifications = append(notifications, NotificationItem{
					ID:        "bonus_" + weekStart.Format("20060102"),
					Type:      "BONUS",
					Title:     "Weekly Volume Bonus Unlocked!",
					Message:   fmt.Sprintf("🎉 Congratulations! You achieved %d total minutes this week. ₹150 Weekly Volume Bonus added to your wallet.", totalMin),
					Timestamp: formatNotificationTimestamp(lastCallTime),
					IsRead:    now.Sub(lastCallTime) > 24*time.Hour,
					IconType:  "bonus",
					CreatedAt: lastCallTime.Format(time.RFC3339),
				})
			}
		}
	}

	// 5. KYC Status notification
	var kycStatus string
	var kycDate time.Time
	_ = s.pool.QueryRow(ctx, "SELECT kyc_status, updated_at FROM listeners WHERE id = $1", pgtype.UUID{Bytes: listenerID, Valid: true}).Scan(&kycStatus, &kycDate)
	if kycStatus == "approved" {
		notifications = append(notifications, NotificationItem{
			ID:        "kyc_approved",
			Type:      "KYC",
			Title:     "KYC Verified & Approved",
			Message:   "Your identity and bank account documents have been verified. You can take calls and receive weekly payouts.",
			Timestamp: formatNotificationTimestamp(kycDate),
			IsRead:    true,
			IconType:  "shield",
			CreatedAt: kycDate.Format(time.RFC3339),
		})
	}

	// Sort notifications newest first
	sort.Slice(notifications, func(i, j int) bool {
		return notifications[i].CreatedAt > notifications[j].CreatedAt
	})

	unreadCount := 0
	for _, n := range notifications {
		if !n.IsRead {
			unreadCount++
		}
	}

	return &NotificationsResponse{
		UnreadCount:   unreadCount,
		Notifications: notifications,
	}, nil
}




