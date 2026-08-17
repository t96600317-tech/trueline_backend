package calls

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

type ZegoTokenProvider struct {
	appID        string
	serverSecret string
}

func NewZegoTokenProvider(appID, serverSecret string) *ZegoTokenProvider {
	return &ZegoTokenProvider{
		appID:        appID,
		serverSecret: serverSecret,
	}
}

type zegoToken04Payload struct {
	AppID      uint32 `json:"app_id"`
	UserID     string `json:"user_id"`
	Nonce      int64  `json:"nonce"`
	CreateTime int64  `json:"ctime"`
	ExpireTime int64  `json:"expire"`
	Payload    string `json:"payload"`
}

// GenerateToken creates an official ZegoCloud Token04 room authentication token
func (p *ZegoTokenProvider) GenerateToken(userID, roomID string, duration time.Duration) (string, error) {
	if p.serverSecret == "" {
		return "", errors.New("zego server secret is empty")
	}

	appIDNum, _ := strconv.ParseUint(p.appID, 10, 32)
	now := time.Now().Unix()
	expire := now + int64(duration.Seconds())

	var nonce int64
	_ = binary.Read(rand.Reader, binary.BigEndian, &nonce)

	roomPayload := map[string]interface{}{
		"room_id": roomID,
		"privilege": map[int]int{
			1: 1, // login
			2: 1, // publish
		},
		"stream_id_list": nil,
	}
	payloadBytes, _ := json.Marshal(roomPayload)

	tokenData := zegoToken04Payload{
		AppID:      uint32(appIDNum),
		UserID:     userID,
		Nonce:      nonce,
		CreateTime: now,
		ExpireTime: expire,
		Payload:    string(payloadBytes),
	}

	plainBytes, err := json.Marshal(tokenData)
	if err != nil {
		return "", fmt.Errorf("failed to encode token data: %w", err)
	}

	// Pad plainBytes with PKCS7 for AES-CBC
	padded := pkcs7Pad(plainBytes, aes.BlockSize)

	// Ensure secret is 32 bytes for AES-256 or 16 bytes for AES-128
	key := []byte(p.serverSecret)
	if len(key) < 32 {
		paddedKey := make([]byte, 32)
		copy(paddedKey, key)
		key = paddedKey
	} else if len(key) > 32 {
		key = key[:32]
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	// Build Token04 binary buffer:
	// [8 bytes expire] + [2 bytes IV len] + [16 bytes IV] + [2 bytes content len] + [content bytes]
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, expire)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(iv)))
	buf.Write(iv)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(encrypted)))
	buf.Write(encrypted)

	token := fmt.Sprintf("04%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
	return token, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}
