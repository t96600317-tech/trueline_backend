package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"

	"math/big"
	"time"

	"trueline-backend/internal/config"
	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthService struct {
	pool         *pgxpool.Pool
	tokenManager *TokenManager
	otpProvider  OTPProvider
	cfg          *config.Config
}

func NewAuthService(pool *pgxpool.Pool, tm *TokenManager, otpProvider OTPProvider, cfg *config.Config) *AuthService {
	if otpProvider == nil {
		otpProvider = NewMockOTPProvider()
	}
	return &AuthService{
		pool:         pool,
		tokenManager: tm,
		otpProvider:  otpProvider,
		cfg:          cfg,
	}
}

type OTPResponse struct {
	Message          string `json:"message"`
	Phone            string `json:"phone"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	MockOTP          string `json:"mock_otp,omitempty"`
}

type AuthVerifyResponse struct {
	Token        string      `json:"token"`
	Role         string      `json:"role"`
	IsNewUser    bool        `json:"is_new_user"`
	User         *db.User    `json:"user,omitempty"`
	Partner      *db.Partner `json:"partner,omitempty"`
	KYCStatus    string      `json:"kyc_status,omitempty"`
}

func (s *AuthService) RequestOTP(ctx context.Context, phone, role string) (*OTPResponse, error) {
	if phone == "" {
		return nil, errors.New("phone number is required")
	}
	if role != "user" && role != "partner" {
		return nil, errors.New("role must be either 'user' or 'partner'")
	}

	otpCode := "123456"
	if !s.cfg.OTPMockMode {
		nBig, err := rand.Int(rand.Reader, big.NewInt(900000))
		if err != nil {
			return nil, fmt.Errorf("failed to generate OTP: %w", err)
		}
		otpCode = fmt.Sprintf("%06d", nBig.Int64()+100000)
	}

	expiresAt := time.Now().Add(5 * time.Minute)

	if s.pool != nil {
		query := `
			INSERT INTO otp_requests (phone, otp_code, role, expires_at)
			VALUES ($1, $2, $3, $4)
		`
		_, err := s.pool.Exec(ctx, query, phone, otpCode, role, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("failed to save OTP request: %w", err)
		}
	}

	// Dispatch OTP via SMS Provider (Mock, Twilio, or MSG91)
	if err := s.otpProvider.SendOTP(ctx, phone, otpCode); err != nil {
		return nil, fmt.Errorf("failed to send OTP SMS: %w", err)
	}

	resp := &OTPResponse{
		Message:          "OTP sent successfully",
		Phone:            phone,
		ExpiresInSeconds: 300,
	}
	if s.cfg.OTPMockMode {
		resp.MockOTP = otpCode
	}

	return resp, nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, phone, otpCode, role string) (*AuthVerifyResponse, error) {
	if phone == "" || otpCode == "" {
		return nil, errors.New("phone and OTP code are required")
	}
	if role != "user" && role != "partner" {
		return nil, errors.New("role must be either 'user' or 'partner'")
	}

	if s.pool != nil {
		var otpID uuid.UUID
		var storedOTP string
		var expiresAt time.Time

		otpQuery := `
			SELECT id, otp_code, expires_at FROM otp_requests
			WHERE phone = $1 AND role = $2 AND verified = FALSE AND expires_at > NOW()
			ORDER BY created_at DESC
			LIMIT 1
		`
		err := s.pool.QueryRow(ctx, otpQuery, phone, role).Scan(&otpID, &storedOTP, &expiresAt)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("invalid or expired OTP")
			}
			return nil, fmt.Errorf("database query error: %w", err)
		}

		if storedOTP != otpCode && (!s.cfg.OTPMockMode || otpCode != "123456") {
			return nil, errors.New("incorrect OTP code")
		}

		_, _ = s.pool.Exec(ctx, `UPDATE otp_requests SET verified = TRUE WHERE id = $1`, otpID)
	} else {
		// In-memory fallback if database pool is not connected yet
		if storedOTP := "123456"; otpCode != storedOTP {
			return nil, errors.New("incorrect OTP code")
		}
	}

	if role == "user" {
		return s.handleUserLogin(ctx, phone)
	} else {
		return s.handlePartnerLogin(ctx, phone)
	}
}

func (s *AuthService) handleUserLogin(ctx context.Context, phone string) (*AuthVerifyResponse, error) {
	var user db.User
	var isNewUser bool

	if s.pool != nil {
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return nil, err
		}
		defer tx.Rollback(ctx)

		query := `SELECT id, phone, language_pref, status, created_at, updated_at FROM users WHERE phone = $1`
		err = tx.QueryRow(ctx, query, phone).Scan(
			&user.ID, &user.Phone, &user.LanguagePref, &user.Status, &user.CreatedAt, &user.UpdatedAt,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			isNewUser = true
			insertUser := `
				INSERT INTO users (phone, language_pref)
				VALUES ($1, 'hi')
				RETURNING id, phone, language_pref, status, created_at, updated_at
			`
			err = tx.QueryRow(ctx, insertUser, phone).Scan(
				&user.ID, &user.Phone, &user.LanguagePref, &user.Status, &user.CreatedAt, &user.UpdatedAt,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			insertWallet := `INSERT INTO wallets (user_id, balance) VALUES ($1, 0.00)`
			_, err = tx.Exec(ctx, insertWallet, user.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to create user wallet: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch user: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	} else {
		user = db.User{
			ID:           uuid.New(),
			Phone:        phone,
			LanguagePref: "hi",
			Status:       "active",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		isNewUser = true
	}

	token, err := s.tokenManager.GenerateToken(user.ID, "user", user.Phone, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &AuthVerifyResponse{
		Token:     token,
		Role:      "user",
		IsNewUser: isNewUser,
		User:      &user,
	}, nil
}

func (s *AuthService) handlePartnerLogin(ctx context.Context, phone string) (*AuthVerifyResponse, error) {
	var partner db.Partner
	var isNewUser bool

	if s.pool != nil {
		query := `
			SELECT id, phone, name, photo_url, bio, languages, rate_per_min, rating_avg, rating_count,
			       kyc_status, status, availability, current_call_session_id, created_at, updated_at
			FROM partners WHERE phone = $1
		`
		err := s.pool.QueryRow(ctx, query, phone).Scan(
			&partner.ID, &partner.Phone, &partner.Name, &partner.PhotoURL, &partner.Bio,
			&partner.Languages, &partner.RatePerMin, &partner.RatingAvg, &partner.RatingCount,
			&partner.KYCStatus, &partner.Status, &partner.Availability, &partner.CurrentCallSessionID,
			&partner.CreatedAt, &partner.UpdatedAt,
		)

		if errors.Is(err, pgx.ErrNoRows) {
			isNewUser = true
			insertPartner := `
				INSERT INTO partners (phone, name, languages, rate_per_min, kyc_status, availability)
				VALUES ($1, '', ARRAY['hi'], 9.00, 'pending', 'offline')
				RETURNING id, phone, name, photo_url, bio, languages, rate_per_min, rating_avg, rating_count,
				          kyc_status, status, availability, current_call_session_id, created_at, updated_at
			`
			err = s.pool.QueryRow(ctx, insertPartner, phone).Scan(
				&partner.ID, &partner.Phone, &partner.Name, &partner.PhotoURL, &partner.Bio,
				&partner.Languages, &partner.RatePerMin, &partner.RatingAvg, &partner.RatingCount,
				&partner.KYCStatus, &partner.Status, &partner.Availability, &partner.CurrentCallSessionID,
				&partner.CreatedAt, &partner.UpdatedAt,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to create listener partner: %w", err)
			}
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch partner: %w", err)
		}
	} else {
		partner = db.Partner{
			ID:           uuid.New(),
			Phone:        phone,
			Name:         "",
			Languages:    []string{"hi"},
			RatePerMin:   9.00,
			KYCStatus:    "pending",
			Availability: "offline",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		isNewUser = true
	}

	token, err := s.tokenManager.GenerateToken(partner.ID, "partner", partner.Phone, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &AuthVerifyResponse{
		Token:     token,
		Role:      "partner",
		IsNewUser: isNewUser,
		Partner:   &partner,
		KYCStatus: partner.KYCStatus,
	}, nil
}
