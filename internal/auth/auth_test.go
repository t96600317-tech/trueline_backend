package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTokenManager_GenerateAndValidate(t *testing.T) {
	secret := "test_secret_key_12345"
	tm := NewTokenManager(secret)

	userID := uuid.New()
	role := "user"
	phone := "+919876543210"

	token, err := tm.GenerateToken(userID, role, phone, 1*time.Hour)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.Role != role {
		t.Errorf("Expected Role %s, got %s", role, claims.Role)
	}
	if claims.Phone != phone {
		t.Errorf("Expected Phone %s, got %s", phone, claims.Phone)
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretPass123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	if !CheckPasswordHash(password, hash) {
		t.Errorf("Password hash check failed for correct password")
	}

	if CheckPasswordHash("WrongPass", hash) {
		t.Errorf("Password hash check succeeded for incorrect password")
	}
}
