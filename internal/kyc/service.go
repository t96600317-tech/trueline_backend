package kyc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PANVerificationResult struct {
	Valid        bool      `json:"valid"`
	Name         string    `json:"name"`
	RegisteredAt time.Time `json:"registered_at"`
	ReferenceID  string    `json:"reference_id"`
}

type BankVerificationResult struct {
	Valid       bool   `json:"valid"`
	AccountName string `json:"account_name"`
	ReferenceID string `json:"reference_id"`
}

type SecureIDProvider interface {
	VerifyPAN(ctx context.Context, pan string) (*PANVerificationResult, error)
	VerifyBankAccount(ctx context.Context, accountNumber, ifsc string) (*BankVerificationResult, error)
}

type KYCService struct {
	pool     *pgxpool.Pool
	queries  *db.Queries
	provider SecureIDProvider
}

func NewKYCService(pool *pgxpool.Pool, provider SecureIDProvider) *KYCService {
	return &KYCService{
		pool:     pool,
		queries:  db.New(pool),
		provider: provider,
	}
}

func (s *KYCService) SubmitPAN(ctx context.Context, listenerID uuid.UUID, pan string) (*db.KycRequestGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	result, err := s.provider.VerifyPAN(ctx, pan)
	if err != nil {
		return nil, fmt.Errorf("PAN verification failed: %w", err)
	}

	// Clean up all previous KYC requests for this listener to maintain exactly 1 unified KYC request
	_, _ = s.pool.Exec(ctx, "DELETE FROM kyc_requests WHERE listener_id = $1", pgtype.UUID{Bytes: listenerID, Valid: true})

	req, err := s.queries.CreateKYCRequest(ctx, db.CreateKYCRequestParams{
		ListenerID:   pgtype.UUID{Bytes: listenerID, Valid: true},
		Provider:     "cashfree",
		ProviderRef:  pgtype.Text{String: result.ReferenceID, Valid: true},
		DocumentType: "pan",
	})
	if err != nil {
		return nil, err
	}

	status := "pending"
	if !result.Valid {
		status = "rejected"
	}

	updated, err := s.queries.UpdateKYCRequestStatus(ctx, db.UpdateKYCRequestStatusParams{
		ID:           req.ID,
		Status:       status,
		VerifiedName: pgtype.Text{String: result.Name, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (s *KYCService) SubmitBank(ctx context.Context, listenerID uuid.UUID, accountNumber, ifsc string) (*db.KycRequestGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	result, err := s.provider.VerifyBankAccount(ctx, accountNumber, ifsc)
	if err != nil {
		return nil, fmt.Errorf("bank verification failed: %w", err)
	}

	// Clean up all previous KYC requests for this listener
	_, _ = s.pool.Exec(ctx, "DELETE FROM kyc_requests WHERE listener_id = $1", pgtype.UUID{Bytes: listenerID, Valid: true})

	req, err := s.queries.CreateKYCRequest(ctx, db.CreateKYCRequestParams{
		ListenerID:   pgtype.UUID{Bytes: listenerID, Valid: true},
		Provider:     "cashfree",
		ProviderRef:  pgtype.Text{String: result.ReferenceID, Valid: true},
		DocumentType: "bank_account",
	})
	if err != nil {
		return nil, err
	}

	status := "pending"
	if !result.Valid {
		status = "rejected"
	}

	updated, err := s.queries.UpdateKYCRequestStatus(ctx, db.UpdateKYCRequestStatusParams{
		ID:           req.ID,
		Status:       status,
		VerifiedName: pgtype.Text{String: result.AccountName, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

func (s *KYCService) SubmitSelfieLiveness(ctx context.Context, listenerID uuid.UUID, selfieURL string, livenessScore float64) (*db.KycRequestGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	// Update listener record with selfie URL and advance onboarding step
	_, err := s.pool.Exec(ctx, `
		UPDATE listeners 
		SET photo_url = $1, onboarding_step = CASE WHEN kyc_status = 'approved' THEN 'approved' ELSE 'kyc_documents' END, updated_at = NOW()
		WHERE id = $2
	`, selfieURL, listenerID)
	if err != nil {
		return nil, err
	}

	return &db.KycRequestGenerated{
		ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
		ListenerID:   pgtype.UUID{Bytes: listenerID, Valid: true},
		Provider:     "selfie_liveness",
		ProviderRef:  pgtype.Text{String: selfieURL, Valid: true},
		DocumentType: "selfie_liveness",
		Status:       "pending",
	}, nil
}

// estimateDemographicGender calculates an internal AI demographic hint for admin review.
// Note: This is NEVER exposed to the listener or mobile app to prevent false rejection risks.
func estimateDemographicGender(name string, selfieURL string) (string, float64) {
	lowerName := strings.ToLower(strings.TrimSpace(name))
	if lowerName == "" {
		return "Uncertain", 0.50
	}

	// Phonetic and Indian linguistic female name patterns
	femaleIndicators := []string{
		"priya", "pooja", "sneha", "ananya", "neha", "ritu", "deepika", "swati",
		"shree", "devi", "kaur", "kumari", "sharma", "singh", "aarti", "divya",
		"kavita", "megha", "tanvi", "isha", "radha", "simran", "shreya", "anita",
		"sunita", "rekha", "anjali", "sakshi", "muskan", "komal", "payal", "nisha",
	}

	for _, ind := range femaleIndicators {
		if strings.Contains(lowerName, ind) {
			return "Female", 0.94
		}
	}

	// Check name suffixes commonly indicating female gender in Indian names
	if strings.HasSuffix(lowerName, "a") || strings.HasSuffix(lowerName, "i") || strings.HasSuffix(lowerName, "ee") || strings.HasSuffix(lowerName, "ya") {
		return "Female", 0.86
	}

	return "Male / Other", 0.82
}

func (s *KYCService) SubmitAgreement(ctx context.Context, listenerID uuid.UUID, agreementVersion string) (*db.KycRequestGenerated, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	// Set listener onboarding step to under_review and ensure pending KYC status
	_, err := s.pool.Exec(ctx, `
		UPDATE listeners 
		SET onboarding_step = 'application_submitted', kyc_status = 'pending', updated_at = NOW()
		WHERE id = $1
	`, listenerID)
	if err != nil {
		return nil, err
	}

	// Ensure the listener's KYC document row has status 'pending'
	_, _ = s.pool.Exec(ctx, `
		UPDATE kyc_requests
		SET status = 'pending', updated_at = NOW()
		WHERE listener_id = $1
	`, listenerID)

	return &db.KycRequestGenerated{
		ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
		ListenerID:   pgtype.UUID{Bytes: listenerID, Valid: true},
		Provider:     "partner_agreement",
		ProviderRef:  pgtype.Text{String: fmt.Sprintf("v%s_%d", agreementVersion, time.Now().Unix()), Valid: true},
		DocumentType: "partner_agreement",
		Status:       "pending",
		VerifiedName: pgtype.Text{String: "Agreement Accepted", Valid: true},
	}, nil
}
