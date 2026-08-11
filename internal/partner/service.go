package partner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PartnerService struct {
	pool *pgxpool.Pool
}

func NewPartnerService(pool *pgxpool.Pool) *PartnerService {
	return &PartnerService{pool: pool}
}

type UpdateProfilePayload struct {
	Name       string   `json:"name"`
	PhotoURL   string   `json:"photo_url"`
	Bio        string   `json:"bio"`
	Languages  []string `json:"languages"`
	RatePerMin float64  `json:"rate_per_min"`
}

type SubmitKYCPayload struct {
	DocumentType string `json:"document_type"` // "aadhaar", "pan", "driving_license", "passport"
	DocumentURL  string `json:"document_url"`
}

type AvailabilityPayload struct {
	Availability string `json:"availability"` // "online" or "offline"
}

func (s *PartnerService) GetPartnerProfile(ctx context.Context, partnerID uuid.UUID) (*db.Partner, error) {
	if s.pool == nil {
		return &db.Partner{
			ID:           partnerID,
			Name:         "Demo Listener Partner",
			Languages:    []string{"hi", "en"},
			RatePerMin:   9.00,
			KYCStatus:    "pending",
			Availability: "offline",
		}, nil
	}

	var partner db.Partner
	query := `
		SELECT id, phone, name, photo_url, bio, languages, rate_per_min, rating_avg, rating_count,
		       kyc_status, status, availability, current_call_session_id, created_at, updated_at
		FROM partners WHERE id = $1
	`
	err := s.pool.QueryRow(ctx, query, partnerID).Scan(
		&partner.ID, &partner.Phone, &partner.Name, &partner.PhotoURL, &partner.Bio,
		&partner.Languages, &partner.RatePerMin, &partner.RatingAvg, &partner.RatingCount,
		&partner.KYCStatus, &partner.Status, &partner.Availability, &partner.CurrentCallSessionID,
		&partner.CreatedAt, &partner.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("partner profile not found")
		}
		return nil, fmt.Errorf("failed to fetch partner: %w", err)
	}

	return &partner, nil
}

func (s *PartnerService) UpdateProfile(ctx context.Context, partnerID uuid.UUID, p UpdateProfilePayload) (*db.Partner, error) {
	if p.Name == "" {
		return nil, errors.New("name cannot be empty")
	}
	if p.RatePerMin <= 0 {
		p.RatePerMin = 9.00 // Default ₹9/min
	}
	if len(p.Languages) == 0 {
		p.Languages = []string{"hi"}
	}

	if s.pool == nil {
		return &db.Partner{
			ID:         partnerID,
			Name:       p.Name,
			PhotoURL:   p.PhotoURL,
			Bio:        p.Bio,
			Languages:  p.Languages,
			RatePerMin: p.RatePerMin,
			KYCStatus:  "pending",
		}, nil
	}

	query := `
		UPDATE partners
		SET name = $2, photo_url = $3, bio = $4, languages = $5, rate_per_min = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING id, phone, name, photo_url, bio, languages, rate_per_min, rating_avg, rating_count,
		          kyc_status, status, availability, current_call_session_id, created_at, updated_at
	`

	var partner db.Partner
	err := s.pool.QueryRow(ctx, query, partnerID, p.Name, p.PhotoURL, p.Bio, p.Languages, p.RatePerMin).Scan(
		&partner.ID, &partner.Phone, &partner.Name, &partner.PhotoURL, &partner.Bio,
		&partner.Languages, &partner.RatePerMin, &partner.RatingAvg, &partner.RatingCount,
		&partner.KYCStatus, &partner.Status, &partner.Availability, &partner.CurrentCallSessionID,
		&partner.CreatedAt, &partner.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update partner profile: %w", err)
	}

	return &partner, nil
}

func (s *PartnerService) SubmitKYCDocument(ctx context.Context, partnerID uuid.UUID, p SubmitKYCPayload) (*db.KYCDocument, error) {
	if p.DocumentURL == "" {
		return nil, errors.New("document_url is required")
	}
	validTypes := map[string]bool{"aadhaar": true, "pan": true, "driving_license": true, "passport": true}
	if !validTypes[p.DocumentType] {
		return nil, errors.New("invalid document_type; must be aadhaar, pan, driving_license, or passport")
	}

	if s.pool == nil {
		return &db.KYCDocument{
			ID:           uuid.New(),
			PartnerID:    partnerID,
			DocumentType: p.DocumentType,
			DocumentURL:  p.DocumentURL,
			ReviewStatus: "pending",
			CreatedAt:    time.Now(),
		}, nil
	}

	var doc db.KYCDocument
	query := `
		INSERT INTO kyc_documents (partner_id, document_type, document_url, review_status)
		VALUES ($1, $2, $3, 'pending')
		RETURNING id, partner_id, document_type, document_url, review_status, rejection_reason, reviewed_by, reviewed_at, created_at
	`
	err := s.pool.QueryRow(ctx, query, partnerID, p.DocumentType, p.DocumentURL).Scan(
		&doc.ID, &doc.PartnerID, &doc.DocumentType, &doc.DocumentURL, &doc.ReviewStatus,
		&doc.RejectionReason, &doc.ReviewedBy, &doc.ReviewedAt, &doc.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit KYC document: %w", err)
	}

	return &doc, nil
}

func (s *PartnerService) SetAvailability(ctx context.Context, partnerID uuid.UUID, availability string) (*db.Partner, error) {
	if availability != "online" && availability != "offline" {
		return nil, errors.New("availability must be either 'online' or 'offline'")
	}

	partner, err := s.GetPartnerProfile(ctx, partnerID)
	if err != nil {
		return nil, err
	}

	// Enforce Business Rule: Cannot go online unless KYC is approved!
	if availability == "online" && partner.KYCStatus != "approved" {
		return nil, errors.New("cannot go online until KYC document has been approved by admin")
	}

	if s.pool == nil {
		partner.Availability = availability
		return partner, nil
	}

	query := `
		UPDATE partners
		SET availability = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING id, phone, name, photo_url, bio, languages, rate_per_min, rating_avg, rating_count,
		          kyc_status, status, availability, current_call_session_id, created_at, updated_at
	`

	var updatedPartner db.Partner
	err = s.pool.QueryRow(ctx, query, partnerID, availability).Scan(
		&updatedPartner.ID, &updatedPartner.Phone, &updatedPartner.Name, &updatedPartner.PhotoURL, &updatedPartner.Bio,
		&updatedPartner.Languages, &updatedPartner.RatePerMin, &updatedPartner.RatingAvg, &updatedPartner.RatingCount,
		&updatedPartner.KYCStatus, &updatedPartner.Status, &updatedPartner.Availability, &updatedPartner.CurrentCallSessionID,
		&updatedPartner.CreatedAt, &updatedPartner.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update availability: %w", err)
	}

	return &updatedPartner, nil
}
