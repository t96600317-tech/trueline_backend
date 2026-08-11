package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	pool *pgxpool.Pool
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{pool: pool}
}

func (s *UserService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*db.User, float64, error) {
	if s.pool == nil {
		return &db.User{
			ID:           userID,
			Phone:        "+919876543210",
			LanguagePref: "hi",
			Status:       "active",
		}, 260.00, nil // 260 Coins default for testing!
	}

	var u db.User
	userQuery := `SELECT id, phone, language_pref, status, created_at, updated_at FROM users WHERE id = $1`
	err := s.pool.QueryRow(ctx, userQuery, userID).Scan(
		&u.ID, &u.Phone, &u.LanguagePref, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, errors.New("user not found")
		}
		return nil, 0, fmt.Errorf("failed to fetch user: %w", err)
	}

	var balance float64
	walletQuery := `SELECT balance FROM wallets WHERE user_id = $1`
	err = s.pool.QueryRow(ctx, walletQuery, userID).Scan(&balance)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		balance = 260.00
	}

	return &u, balance, nil
}

func (s *UserService) UpdateLanguage(ctx context.Context, userID uuid.UUID, language string) (*db.User, error) {
	if language == "" {
		return nil, errors.New("language is required")
	}

	if s.pool == nil {
		return &db.User{
			ID:           userID,
			LanguagePref: language,
		}, nil
	}

	var u db.User
	query := `
		UPDATE users
		SET language_pref = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, phone, language_pref, status, created_at, updated_at
	`
	err := s.pool.QueryRow(ctx, query, userID, language).Scan(
		&u.ID, &u.Phone, &u.LanguagePref, &u.Status, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update language preference: %w", err)
	}

	return &u, nil
}

func (s *UserService) ListDiscoverPartners(ctx context.Context, userID uuid.UUID, language, searchQuery string) ([]db.Partner, error) {
	if s.pool == nil {
		// Mock Partners matching UI design screenshot
		mockList := []db.Partner{
			{
				ID:             uuid.MustParse("a0000000-0000-0000-0000-000000000001"),
				Name:           "Afreen",
				Title:          "Joy Helper",
				PhotoURL:       "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80",
				AudioSampleURL: "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
				Bio:            "Always here to listen and bring joy to your day.",
				Languages:      []string{"Hindi", "Bengali"},
				RatePerMin:     11.00,
				RatingAvg:      4.50,
				RatingCount:    38,
				KYCStatus:      "approved",
				Availability:   "online",
				IsFavourite:    true,
			},
			{
				ID:             uuid.MustParse("a0000000-0000-0000-0000-000000000002"),
				Name:           "Ahmedi",
				Title:          "Calm Friend",
				PhotoURL:       "https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80",
				AudioSampleURL: "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
				Bio:            "A calm and patient listener for work and personal chats.",
				Languages:      []string{"Urdu", "Hindi"},
				RatePerMin:     11.00,
				RatingAvg:      4.80,
				RatingCount:    54,
				KYCStatus:      "approved",
				Availability:   "online",
				IsFavourite:    false,
			},
			{
				ID:             uuid.MustParse("a0000000-0000-0000-0000-000000000003"),
				Name:           "Saima",
				Title:          "Calm Friend",
				PhotoURL:       "https://images.unsplash.com/photo-1524504388940-b1c1722653e1?auto=format&fit=crop&w=400&q=80",
				AudioSampleURL: "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
				Bio:            "Fluent in English and Spanish. Safe space to vent.",
				Languages:      []string{"English", "Spanish"},
				RatePerMin:     11.00,
				RatingAvg:      4.80,
				RatingCount:    29,
				KYCStatus:      "approved",
				Availability:   "online",
				IsFavourite:    false,
			},
			{
				ID:             uuid.MustParse("a0000000-0000-0000-0000-000000000004"),
				Name:           "Nirvi",
				Title:          "Happy Coach",
				PhotoURL:       "https://images.unsplash.com/photo-1494790108377-be9c29b29330?auto=format&fit=crop&w=400&q=80",
				AudioSampleURL: "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
				Bio:            "Positivity coach. Talk whenever you feel down.",
				Languages:      []string{"English", "Hindi"},
				RatePerMin:     11.00,
				RatingAvg:      4.90,
				RatingCount:    61,
				KYCStatus:      "approved",
				Availability:   "online",
				IsFavourite:    false,
			},
			{
				ID:             uuid.MustParse("a0000000-0000-0000-0000-000000000005"),
				Name:           "Pooja Sharma",
				Title:          "Desi Companion",
				PhotoURL:       "https://images.unsplash.com/photo-1488426862026-3ee34a7d66df?auto=format&fit=crop&w=400&q=80",
				AudioSampleURL: "https://actions.google.com/sounds/v1/ambiences/rain_heavy.ogg",
				Bio:            "Bhojpuri & Hindi listener. Warm conversations.",
				Languages:      []string{"Bhojpuri", "Hindi"},
				RatePerMin:     11.00,
				RatingAvg:      4.70,
				RatingCount:    19,
				KYCStatus:      "approved",
				Availability:   "online",
				IsFavourite:    false,
			},
		}

		filtered := make([]db.Partner, 0)
		for _, p := range mockList {
			if searchQuery != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(searchQuery)) {
				continue
			}
			if language != "" && language != "All" {
				matched := false
				for _, l := range p.Languages {
					if strings.EqualFold(l, language) {
						matched = true
						break
					}
				}
				if !matched {
					continue
				}
			}
			filtered = append(filtered, p)
		}
		return filtered, nil
	}

	query := `
		SELECT p.id, p.phone, p.name, p.title, p.photo_url, p.audio_sample_url, p.bio, p.languages,
		       p.rate_per_min, p.rating_avg, p.rating_count, p.kyc_status, p.status, p.availability,
		       p.current_call_session_id, p.created_at, p.updated_at,
		       CASE WHEN f.id IS NOT NULL THEN true ELSE false END as is_favourite
		FROM partners p
		LEFT JOIN favourites f ON f.partner_id = p.id AND f.user_id = $1
		WHERE p.status = 'active' AND p.kyc_status = 'approved'
		ORDER BY 
		    CASE WHEN p.availability = 'online' THEN 1 ELSE 2 END,
		    p.rating_avg DESC
	`
	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list discover partners: %w", err)
	}
	defer rows.Close()

	partners := make([]db.Partner, 0)
	for rows.Next() {
		var p db.Partner
		err := rows.Scan(
			&p.ID, &p.Phone, &p.Name, &p.Title, &p.PhotoURL, &p.AudioSampleURL, &p.Bio, &p.Languages, &p.RatePerMin,
			&p.RatingAvg, &p.RatingCount, &p.KYCStatus, &p.Status, &p.Availability,
			&p.CurrentCallSessionID, &p.CreatedAt, &p.UpdatedAt, &p.IsFavourite,
		)
		if err != nil {
			return nil, err
		}

		if searchQuery != "" && !strings.Contains(strings.ToLower(p.Name), strings.ToLower(searchQuery)) {
			continue
		}

		if language != "" && language != "All" {
			matched := false
			for _, lang := range p.Languages {
				if strings.EqualFold(lang, language) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		partners = append(partners, p)
	}

	return partners, nil
}
