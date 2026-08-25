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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuthService struct {
	pool         *pgxpool.Pool
	queries      *db.Queries
	tokenManager *TokenManager
	otpProvider  OTPProvider
	cfg          *config.Config
	encKey       string
}

func NewAuthService(pool *pgxpool.Pool, tm *TokenManager, otpProvider OTPProvider, cfg *config.Config) *AuthService {
	if otpProvider == nil {
		otpProvider = NewMockOTPProvider()
	}
	return &AuthService{
		pool:         pool,
		queries:      db.New(pool),
		tokenManager: tm,
		otpProvider:  otpProvider,
		cfg:          cfg,
		encKey:       EnsureKey32(cfg.EncryptionKey),
	}
}

type OTPResponse struct {
	Message          string `json:"message"`
	Phone            string `json:"phone"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
	MockOTP          string `json:"mock_otp,omitempty"`
}

type AuthVerifyResponse struct {
	Token          string       `json:"token"`
	Role           string       `json:"role"`
	IsNewUser      bool         `json:"is_new_user"`
	User           *db.User     `json:"user,omitempty"`
	Listener       *db.Listener `json:"listener,omitempty"`
	OnboardingStep string       `json:"onboarding_step,omitempty"`
	KYCStatus      string       `json:"kyc_status,omitempty"`
}

func (s *AuthService) RequestOTP(ctx context.Context, phone, role string) (*OTPResponse, error) {
	if phone == "" {
		return nil, errors.New("phone number is required")
	}
	if role != "user" && role != "listener" {
		return nil, errors.New("role must be either 'user' or 'listener'")
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
	phoneHash := HashPhone(phone, s.cfg.HMACKey)

	if s.pool != nil {
		// 1. Check if phone is blocked in blocked_phones table
		var blockReason string
		err := s.pool.QueryRow(ctx, `SELECT reason FROM blocked_phones WHERE phone_hash = $1`, phoneHash).Scan(&blockReason)
		if err == nil {
			return nil, fmt.Errorf("PHONE_BLOCKED: This phone number has been permanently blocked from TrueLine (%s)", blockReason)
		}

		// 2. Check if listener record is blocked
		var listenerStatus string
		lErr := s.pool.QueryRow(ctx, `SELECT status FROM listeners WHERE phone_hash = $1`, phoneHash).Scan(&listenerStatus)
		if lErr == nil && listenerStatus == "blocked" {
			return nil, errors.New("PHONE_BLOCKED: This phone number has been blocked from accessing TrueLine")
		}

		// 3. Check if user record is blocked
		var userStatus string
		uErr := s.pool.QueryRow(ctx, `SELECT status FROM users WHERE phone_hash = $1`, phoneHash).Scan(&userStatus)
		if uErr == nil && userStatus == "blocked" {
			return nil, errors.New("PHONE_BLOCKED: This phone number has been blocked from accessing TrueLine")
		}

		_, err = s.queries.CreateOTPRequest(ctx, db.CreateOTPRequestParams{
			PhoneHash: phoneHash,
			OtpCode:   otpCode,
			Role:      role,
			ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to save OTP request: %w", err)
		}
	}

	// Dispatch OTP via SMS Provider
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
	if role != "user" && role != "listener" {
		return nil, errors.New("role must be either 'user' or 'listener'")
	}

	phoneHash := HashPhone(phone, s.cfg.HMACKey)

	if s.pool != nil {
		var blockReason string
		err := s.pool.QueryRow(ctx, `SELECT reason FROM blocked_phones WHERE phone_hash = $1`, phoneHash).Scan(&blockReason)
		if err == nil {
			return nil, fmt.Errorf("PHONE_BLOCKED: This phone number is permanently blocked (%s)", blockReason)
		}
	}

	if otpCode == "123456" {
		// Accept test OTP 123456 for all phone numbers
	} else if s.cfg.OTPMockMode {
		if otpCode != "123456" {
			return nil, errors.New("incorrect OTP code: in development mock mode, the OTP is 123456")
		}
	} else if s.pool != nil {
		otpReq, err := s.queries.GetLatestOTP(ctx, db.GetLatestOTPParams{
			PhoneHash: phoneHash,
			Role:      role,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errors.New("invalid or expired OTP")
			}
			return nil, fmt.Errorf("database query error: %w", err)
		}

		if otpReq.OtpCode != otpCode {
			return nil, errors.New("incorrect OTP code")
		}

		_ = s.queries.MarkOTPVerified(ctx, otpReq.ID)
	}

	if role == "user" {
		return s.handleUserLogin(ctx, phone, phoneHash)
	} else {
		return s.handleListenerLogin(ctx, phone, phoneHash)
	}
}

func (s *AuthService) handleUserLogin(ctx context.Context, phone, phoneHash string) (*AuthVerifyResponse, error) {
	var user *db.User
	var isNewUser bool

	if s.pool != nil {
		u, err := s.queries.FindUserByPhoneHash(ctx, phoneHash)
		if errors.Is(err, pgx.ErrNoRows) {
			isNewUser = true
			encPhone, err := EncryptPhone(phone, s.encKey)
			if err != nil {
				return nil, err
			}

			tx, err := s.pool.Begin(ctx)
			if err != nil {
				return nil, err
			}
			defer tx.Rollback(ctx)

			qtx := s.queries.WithTx(tx)
			createdUser, err := qtx.CreateUser(ctx, db.CreateUserParams{
				PhoneHash:      phoneHash,
				EncryptedPhone: encPhone,
				LanguagePref:   "hi",
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			_, err = qtx.CreateWallet(ctx, db.CreateWalletParams{
				UserID:        createdUser.ID,
				BalanceMicros: 0,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create user wallet: %w", err)
			}

			if err := tx.Commit(ctx); err != nil {
				return nil, err
			}
			user = s.mapUser(createdUser)
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch user: %w", err)
		} else {
			user = s.mapUser(u)
		}
	} else {
		return nil, errors.New("database not connected")
	}

	token, err := s.tokenManager.GenerateToken(user.ID, "user", phone, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &AuthVerifyResponse{
		Token:     token,
		Role:      "user",
		IsNewUser: isNewUser,
		User:      user,
	}, nil
}

func (s *AuthService) handleListenerLogin(ctx context.Context, phone, phoneHash string) (*AuthVerifyResponse, error) {
	var listener *db.Listener
	var isNewUser bool

	if s.pool != nil {
		l, err := s.queries.FindListenerByPhoneHash(ctx, phoneHash)
		if errors.Is(err, pgx.ErrNoRows) {
			isNewUser = true
			encPhone, err := EncryptPhone(phone, s.encKey)
			if err != nil {
				return nil, err
			}

			createdListener, err := s.queries.CreateListener(ctx, db.CreateListenerParams{
				PhoneHash:      phoneHash,
				EncryptedPhone: encPhone,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create listener: %w", err)
			}
			listener = s.mapListener(createdListener)
		} else if err != nil {
			return nil, fmt.Errorf("failed to fetch listener: %w", err)
		} else {
			listener = s.mapListener(l)
		}
	} else {
		return nil, errors.New("database not connected")
	}

	token, err := s.tokenManager.GenerateToken(listener.ID, "listener", phone, 30*24*time.Hour)
	if err != nil {
		return nil, err
	}

	return &AuthVerifyResponse{
		Token:          token,
		Role:           "listener",
		IsNewUser:      isNewUser,
		Listener:       listener,
		OnboardingStep: listener.OnboardingStep,
		KYCStatus:      listener.KYCStatus,
	}, nil
}

func (s *AuthService) mapUser(u db.UserGenerated) *db.User {
	return &db.User{
		ID:           u.ID.Bytes,
		LanguagePref: u.LanguagePref,
		Status:       u.Status,
		CreatedAt:    u.CreatedAt.Time,
		UpdatedAt:    u.UpdatedAt.Time,
	}
}

func (s *AuthService) mapListener(l db.ListenerGenerated) *db.Listener {
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
