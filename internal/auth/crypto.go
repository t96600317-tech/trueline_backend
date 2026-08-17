package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
)

// HashPhone generates a stable HMAC-SHA256 hash of the phone number for lookup.
func HashPhone(phone, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(phone))
	return hex.EncodeToString(h.Sum(nil))
}

// EncryptPhone encrypts the phone number using AES-GCM.
func EncryptPhone(phone, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(phone), nil)
	return hex.EncodeToString(ciphertext), nil
}

// DecryptPhone decrypts the phone number using AES-GCM.
func DecryptPhone(encryptedPhone, key string) (string, error) {
	data, err := hex.DecodeString(encryptedPhone)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

func EnsureKey32(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])[:32]
}
