package calls

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncomingCallNotifier deliberately has no client-side credentials. The app
// registers only its APNs device token; the APNs signing key stays on the server.
type IncomingCallNotifier interface {
	NotifyIncomingCall(ctx context.Context, listenerID uuid.UUID, sessionID uuid.UUID, callerName string)
}

type APNsVoIPNotifier struct {
	pool       *pgxpool.Pool
	teamID     string
	keyID      string
	bundleID   string
	privateKey *ecdsa.PrivateKey
	endpoint   string
	httpClient *http.Client
}

func NewAPNsVoIPNotifier(
	pool *pgxpool.Pool,
	teamID, keyID, bundleID, privateKeyPEM string,
	sandbox bool,
) (*APNsVoIPNotifier, error) {
	if pool == nil {
		return nil, errors.New("database is not connected")
	}
	if teamID == "" || keyID == "" || bundleID == "" || privateKeyPEM == "" {
		return nil, errors.New("APNs team ID, key ID, bundle ID, and private key are all required")
	}
	decodedPEM := strings.ReplaceAll(strings.TrimSpace(privateKeyPEM), `\\n`, "\n")
	block, _ := pem.Decode([]byte(decodedPEM))
	if block == nil {
		return nil, errors.New("APNs private key is not valid PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse APNs private key: %w", err)
	}
	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("APNs private key must be an EC PKCS#8 key")
	}
	endpoint := "https://api.push.apple.com"
	if sandbox {
		endpoint = "https://api.sandbox.push.apple.com"
	}
	return &APNsVoIPNotifier{
		pool:       pool,
		teamID:     teamID,
		keyID:      keyID,
		bundleID:   bundleID,
		privateKey: ecdsaKey,
		endpoint:   endpoint,
		httpClient: &http.Client{Timeout: 8 * time.Second},
	}, nil
}

func (n *APNsVoIPNotifier) NotifyIncomingCall(ctx context.Context, listenerID uuid.UUID, sessionID uuid.UUID, callerName string) {
	rows, err := n.pool.Query(ctx, `
		SELECT device_token
		FROM listener_ios_voip_devices
		WHERE listener_id = $1
	`, listenerID)
	if err != nil {
		log.Printf("APNs VoIP token lookup failed for listener %s: %v", listenerID, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			continue
		}
		if err := n.send(ctx, token, sessionID.String(), callerName); err != nil {
			log.Printf("APNs VoIP delivery failed for listener %s: %v", listenerID, err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("APNs VoIP token iteration failed for listener %s: %v", listenerID, err)
	}
}

func (n *APNsVoIPNotifier) send(ctx context.Context, deviceToken, sessionID, callerName string) error {
	authorization, err := n.authorizationToken()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"aps":         map[string]any{"content-available": 1},
		"session_id":  sessionID,
		"caller_name": strings.TrimSpace(callerName),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		n.endpoint+"/3/device/"+deviceToken,
		bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("authorization", "bearer "+authorization)
	req.Header.Set("apns-topic", n.bundleID+".voip")
	req.Header.Set("apns-push-type", "voip")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-expiration", "0")
	req.Header.Set("content-type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("APNs returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func (n *APNsVoIPNotifier) authorizationToken() (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": n.teamID,
		"iat": time.Now().Unix(),
	})
	token.Header["kid"] = n.keyID
	return token.SignedString(n.privateKey)
}
