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
	"math/big"
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
	CreateTime int64  `json:"ctime"`
	ExpireTime int64  `json:"expire"`
	Nonce      int32  `json:"nonce"`
	Payload    string `json:"payload"`
}

// GenerateToken creates a ZegoCloud Token04 room authentication token. Its
// binary envelope and encrypted payload match Zego's server-assistant format.
func (p *ZegoTokenProvider) GenerateToken(userID, roomID string, duration time.Duration) (string, error) {
	if userID == "" {
		return "", errors.New("zego user ID is empty")
	}
	if roomID == "" {
		return "", errors.New("zego room ID is empty")
	}
	if duration <= 0 {
		return "", errors.New("zego token duration must be positive")
	}
	if len(p.serverSecret) != 32 {
		return "", errors.New("zego server secret must be exactly 32 bytes")
	}

	appIDNum, err := strconv.ParseUint(p.appID, 10, 32)
	if err != nil || appIDNum == 0 {
		return "", errors.New("zego app ID must be a non-zero unsigned 32-bit integer")
	}

	now := time.Now().Unix()
	expire := now + int64(duration.Seconds())

	nonce, err := rand.Int(rand.Reader, big.NewInt(1<<31))
	if err != nil {
		return "", fmt.Errorf("failed to generate zego token nonce: %w", err)
	}

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
		CreateTime: now,
		ExpireTime: expire,
		Nonce:      int32(nonce.Int64()),
		Payload:    string(payloadBytes),
	}

	plainBytes, err := json.Marshal(tokenData)
	if err != nil {
		return "", fmt.Errorf("failed to encode token data: %w", err)
	}

	// Token04 uses AES-CBC with PKCS#5/#7 padding.
	padded := pkcs7Pad(plainBytes, aes.BlockSize)

	block, err := aes.NewCipher([]byte(p.serverSecret))
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
