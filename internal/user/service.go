package user

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewUserService(pool *pgxpool.Pool) *UserService {
	return &UserService{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (s *UserService) GetUserProfile(ctx context.Context, userID uuid.UUID) (*db.User, int64, error) {
	if s.pool == nil {
		return nil, 0, errors.New("database not connected")
	}

	u, err := s.queries.GetUserByID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, errors.New("user not found")
		}
		return nil, 0, fmt.Errorf("failed to fetch user: %w", err)
	}

	w, err := s.queries.GetWalletByUserID(ctx, pgtype.UUID{Bytes: userID, Valid: true})
	balance := int64(0)
	if err == nil {
		balance = w.BalanceMicros
	}

	return s.mapUser(u), balance, nil
}

func (s *UserService) UpdateLanguage(ctx context.Context, userID uuid.UUID, language string) (*db.User, error) {
	if language == "" {
		return nil, errors.New("language is required")
	}

	u, err := s.queries.UpdateUserLanguage(ctx, db.UpdateUserLanguageParams{
		ID:           pgtype.UUID{Bytes: userID, Valid: true},
		LanguagePref: language,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to update language preference: %w", err)
	}

	return s.mapUser(u), nil
}

func (s *UserService) ListDiscoverListeners(ctx context.Context, userID uuid.UUID, language, searchQuery string) ([]db.Listener, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	rows, err := s.queries.ListAllListeners(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list discover listeners: %w", err)
	}

	listeners := make([]db.Listener, 0)
	for _, l := range rows {
		if searchQuery != "" && !strings.Contains(strings.ToLower(l.Name), strings.ToLower(searchQuery)) {
			continue
		}

		if language != "" && language != "All" {
			matched := false
			for _, lang := range l.Languages {
				if strings.EqualFold(lang, language) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		listeners = append(listeners, *s.mapListener(l))
	}

	return listeners, nil
}

func (s *UserService) mapUser(u db.UserGenerated) *db.User {
	return &db.User{
		ID:           u.ID.Bytes,
		LanguagePref: u.LanguagePref,
		Status:       u.Status,
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

func (s *UserService) mapListener(l db.ListenerGenerated) *db.Listener {
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
		PhotoURL:             l.PhotoUrl,
		AudioSampleURL:       l.AudioSampleUrl,
		Bio:                  l.Bio,
		Languages:            l.Languages,
		RatePerMinMicros:     l.RatePerMinMicros,
		EarningPerMinMicros:  l.EarningPerMinMicros,
		RatingAvg:            ratingAvg.Float64,
		RatingCount:          int(l.RatingCount),
		OnboardingStep:       l.OnboardingStep,
		KYCStatus:            l.KycStatus,
		Status:               l.Status,
		Availability:         l.Availability,
		CurrentCallSessionID: currentCallID,
		CreatedAt:            l.CreatedAt.Time,
		UpdatedAt:            l.UpdatedAt.Time,
	}
}
