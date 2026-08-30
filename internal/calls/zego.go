package calls

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
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

// ConfigurationFingerprint identifies the active Zego configuration without
// revealing the server secret. It is useful when diagnosing a rejected token
// issued by a remote deployment.
func (p *ZegoTokenProvider) ConfigurationFingerprint() string {
	sum := sha256.Sum256([]byte(p.appID + "\x00" + p.serverSecret))
	return hex.EncodeToString(sum[:8])
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

	block, err := aes.NewCipher([]byte(p.serverSecret))
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Token04 is AES-GCM encrypted. AES-CBC was used by obsolete Token04
	// implementations and produces credentials the current Zego SDK rejects.
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM cipher: %w", err)
	}
	gcmNonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, gcmNonce); err != nil {
		return "", fmt.Errorf("failed to generate GCM nonce: %w", err)
	}
	encrypted := gcm.Seal(nil, gcmNonce, plainBytes, nil)

	// Build the current Token04 binary envelope, matching Zego's official
	// server assistant: expiry, nonce, encrypted payload, AES-GCM mode byte.
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.BigEndian, expire)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(gcmNonce)))
	buf.Write(gcmNonce)
	_ = binary.Write(buf, binary.BigEndian, uint16(len(encrypted)))
	buf.Write(encrypted)
	_ = binary.Write(buf, binary.BigEndian, uint8(1)) // AES-GCM

	token := fmt.Sprintf("04%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
	return token, nil
}
