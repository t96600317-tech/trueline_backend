package db

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `json:"id"`
	Phone        string    `json:"phone"`
	LanguagePref string    `json:"language_pref"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Partner struct {
	ID                   uuid.UUID  `json:"id"`
	Phone                string     `json:"phone"`
	Name                 string     `json:"name"`
	Title                string     `json:"title"`            // e.g. "Joy Helper", "Calm Friend"
	PhotoURL             string     `json:"photo_url"`
	AudioSampleURL       string     `json:"audio_sample_url"` // Voice intro preview
	Bio                  string     `json:"bio"`
	Languages            []string   `json:"languages"`
	RatePerMin           float64    `json:"rate_per_min"`
	RatingAvg            float64    `json:"rating_avg"`
	RatingCount          int        `json:"rating_count"`
	KYCStatus            string     `json:"kyc_status"`       // pending, approved, rejected
	Status               string     `json:"status"`           // active, blocked
	Availability         string     `json:"availability"`     // online, offline
	IsFavourite          bool       `json:"is_favourite"`     // Dynamic flag for user Discover screen
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

type KYCDocument struct {
	ID              uuid.UUID  `json:"id"`
	PartnerID       uuid.UUID  `json:"partner_id"`
	DocumentType    string     `json:"document_type"`
	DocumentURL     string     `json:"document_url"`
	ReviewStatus    string     `json:"review_status"`
	RejectionReason *string    `json:"rejection_reason,omitempty"`
	ReviewedBy      *uuid.UUID `json:"reviewed_by,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

type OTPRequest struct {
	ID        uuid.UUID `json:"id"`
	Phone     string    `json:"phone"`
	OTPCode   string    `json:"-"`
	Role      string    `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
}

type Wallet struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Balance   float64   `json:"balance"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WalletTransaction struct {
	ID             uuid.UUID `json:"id"`
	WalletID       uuid.UUID `json:"wallet_id"`
	Type           string    `json:"type"`
	Amount         float64   `json:"amount"`
	BalanceAfter   float64   `json:"balance_after"`
	ReferenceID    string    `json:"reference_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
}

type CallSession struct {
	ID                 uuid.UUID  `json:"id"`
	UserID             uuid.UUID  `json:"user_id"`
	PartnerID          uuid.UUID  `json:"partner_id"`
	Provider           string     `json:"provider"`
	RoomID             string     `json:"room_id"`
	ZegoTokenRef       string     `json:"zego_token_ref"`
	Status             string     `json:"status"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	EndedAt            *time.Time `json:"ended_at,omitempty"`
	EndReason          *string    `json:"end_reason,omitempty"`
	RatePerMinSnapshot float64    `json:"rate_per_min_snapshot"`
	CreatedAt          time.Time  `json:"created_at"`
}

type ChatMessage struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	PartnerID  uuid.UUID  `json:"partner_id"`
	SenderType string     `json:"sender_type"` // "user" or "partner"
	Content    string     `json:"content"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ConversationSummary struct {
	PartnerID         uuid.UUID `json:"partner_id"`
	PartnerName       string    `json:"partner_name"`
	PartnerTitle      string    `json:"partner_title"`
	PartnerPhotoURL   string    `json:"partner_photo_url"`
	PartnerAvailability string  `json:"partner_availability"` // "online" or "offline"
	LastMessage       string    `json:"last_message"`
	LastMessageSender string    `json:"last_message_sender"`
	LastMessageTime   time.Time `json:"last_message_time"`
	UnreadCount       int       `json:"unread_count"`
}
