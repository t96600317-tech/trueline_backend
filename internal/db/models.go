package db

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	PhoneHash      string    `json:"-"`
	EncryptedPhone string    `json:"-"`
	LanguagePref   string    `json:"language_pref"`
	Status         string    `json:"status"` // active, blocked
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Listener struct {
	ID                   uuid.UUID  `json:"id"`
	Name                 string     `json:"name"`
	Title                string     `json:"title"`
	PhotoURL             string     `json:"photo_url"`
	AudioSampleURL       string     `json:"audio_sample_url"`
	Bio                  string     `json:"bio"`
	Languages            []string   `json:"languages"`
	RatePerMinMicros     int64      `json:"rate_per_min_micros"`
	EarningPerMinMicros  int64      `json:"earning_per_min_micros"`
	RatingAvg            float64    `json:"rating_avg"`
	RatingCount          int        `json:"rating_count"`
	OnboardingStep       string     `json:"onboarding_step"`
	KYCStatus            string     `json:"kyc_status"`   // pending, approved, rejected
	Status               string     `json:"status"`       // active, blocked
	Availability         string     `json:"availability"` // online, offline
	IsFavourite          bool       `json:"is_favourite"`
	CurrentCallSessionID *uuid.UUID `json:"current_call_session_id,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type Admin struct {
	ID           uuid.UUID `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Name         string    `json:"name"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

type Wallet struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	BalanceMicros int64      `json:"balance_micros"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type WalletLedgerEntry struct {
	ID                 uuid.UUID `json:"id"`
	WalletID           uuid.UUID `json:"wallet_id"`
	Type               string    `json:"type"`
	AmountMicros       int64      `json:"amount_micros"`
	BalanceAfterMicros int64      `json:"balance_after_micros"`
	ReferenceID        string    `json:"reference_id"`
	IdempotencyKey     string    `json:"idempotency_key"`
	Description        string    `json:"description"`
	CreatedAt          time.Time `json:"created_at"`
}

type EarningsLedgerEntry struct {
	ID                 uuid.UUID `json:"id"`
	ListenerID         uuid.UUID `json:"listener_id"`
	Type               string    `json:"type"`
	AmountMicros       int64      `json:"amount_micros"`
	BalanceAfterMicros int64      `json:"balance_after_micros"`
	ReferenceID        string    `json:"reference_id"`
	IdempotencyKey     string    `json:"idempotency_key"`
	TaxInfo            any       `json:"tax_info"`
	CreatedAt          time.Time `json:"created_at"`
}

type CallSession struct {
	ID                          uuid.UUID  `json:"id"`
	UserID                      uuid.UUID  `json:"user_id"`
	ListenerID                  uuid.UUID  `json:"listener_id"`
	Provider                    string     `json:"provider"`
	RoomID                      string     `json:"room_id"`
	Status                      string     `json:"status"`
	StartedAt                   *time.Time `json:"started_at,omitempty"`
	EndedAt                     *time.Time `json:"ended_at,omitempty"`
	EndReason                   *string    `json:"end_reason,omitempty"`
	RatePerMinMicrosSnapshot    int64      `json:"rate_per_min_micros_snapshot"`
	EarningPerMinMicrosSnapshot int64      `json:"earning_per_min_micros_snapshot"`
	CreatedAt                   time.Time  `json:"created_at"`
}

type ChatMessage struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	ListenerID       uuid.UUID  `json:"listener_id"`
	PartnerID        uuid.UUID  `json:"partner_id"`
	SenderType       string     `json:"sender_type"` // "user" or "listener"
	Content          string     `json:"content"`
	ModerationStatus string     `json:"moderation_status"`
	ReadAt           *time.Time `json:"read_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

type ConversationSummary struct {
	PartnerID            uuid.UUID  `json:"partner_id"`
	PartnerName          string     `json:"partner_name"`
	PartnerTitle         string     `json:"partner_title"`
	PartnerPhotoURL      string     `json:"partner_photo_url"`
	PartnerAvailability string     `json:"partner_availability"`
	ListenerID           uuid.UUID  `json:"listener_id"`
	ListenerName         string     `json:"listener_name"`
	ListenerTitle        string     `json:"listener_title"`
	ListenerPhotoURL     string     `json:"listener_photo_url"`
	ListenerAvailability string     `json:"listener_availability"`
	UserID               *uuid.UUID `json:"user_id,omitempty"`
	UserName             string     `json:"user_name,omitempty"`
	UserTitle            string     `json:"user_title,omitempty"`
	UserPhotoURL         string     `json:"user_photo_url,omitempty"`
	UserAvailability     string     `json:"user_availability,omitempty"`
	LastMessage          string     `json:"last_message"`
	LastMessageSender    string     `json:"last_message_sender"`
	LastMessageTime      time.Time  `json:"last_message_time"`
	UnreadCount          int        `json:"unread_count"`
}
