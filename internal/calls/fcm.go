package calls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/option"
)

// FCMNotifier sends Android incoming-call data notifications. The Firebase
// service-account JSON remains server-side and is never sent to either app.
type FCMNotifier struct {
	pool   *pgxpool.Pool
	client *messaging.Client
}

// EnsureAndroidFCMDeviceStore supports Render deployments, which do not run
// SQL migration files automatically. It is safe to invoke on every startup.
func EnsureAndroidFCMDeviceStore(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database is not connected")
	}
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS listener_android_fcm_devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			listener_id UUID NOT NULL REFERENCES listeners(id) ON DELETE CASCADE,
			device_token TEXT NOT NULL UNIQUE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS listener_android_fcm_devices_listener_id_idx
			ON listener_android_fcm_devices(listener_id);
	`)
	if err != nil {
		return fmt.Errorf("ensure Android FCM device store: %w", err)
	}
	return nil
}

func EnsureUserAndroidFCMDeviceStore(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return errors.New("database is not connected")
	}
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_android_fcm_devices (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			device_token TEXT NOT NULL UNIQUE,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS user_android_fcm_devices_user_id_idx
			ON user_android_fcm_devices(user_id);
		ALTER TABLE wallet_ledger DROP CONSTRAINT IF EXISTS wallet_ledger_type_check;
		ALTER TABLE wallet_ledger ADD CONSTRAINT wallet_ledger_type_check 
			CHECK (type IN ('recharge', 'call_debit', 'chat_debit', 'chat_message', 'refund', 'admin_adjustment'));
	`)
	if err != nil {
		return fmt.Errorf("ensure User Android FCM device store: %w", err)
	}
	return nil
}

func NewFCMNotifier(pool *pgxpool.Pool, serviceAccountJSON string) (*FCMNotifier, error) {
	if pool == nil {
		return nil, errors.New("database is not connected")
	}
	if strings.TrimSpace(serviceAccountJSON) == "" {
		return nil, errors.New("Firebase service-account JSON is required")
	}
	app, err := firebase.NewApp(
		context.Background(),
		nil,
		option.WithCredentialsJSON([]byte(serviceAccountJSON)),
	)
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase Admin: %w", err)
	}
	client, err := app.Messaging(context.Background())
	if err != nil {
		return nil, fmt.Errorf("initialize Firebase Cloud Messaging: %w", err)
	}
	return &FCMNotifier{pool: pool, client: client}, nil
}

func (n *FCMNotifier) NotifyIncomingCall(ctx context.Context, listenerID uuid.UUID, sessionID uuid.UUID, callerName string) {
	if n == nil || n.client == nil || n.pool == nil {
		return
	}
	rows, err := n.pool.Query(ctx, `
		SELECT device_token
		FROM listener_android_fcm_devices
		WHERE listener_id = $1
	`, listenerID)
	if err != nil {
		log.Printf("FCM token lookup failed for listener %s: %v", listenerID, err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			continue
		}
		_, err := n.client.Send(ctx, &messaging.Message{
			Token: token,
			Data: map[string]string{
				"type":        "incoming_call",
				"session_id":  sessionID.String(),
				"caller_name": strings.TrimSpace(callerName),
			},
			Android: &messaging.AndroidConfig{Priority: "high"},
		})
		if err != nil {
			log.Printf("FCM delivery failed for listener %s: %v", listenerID, err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("FCM token iteration failed for listener %s: %v", listenerID, err)
	}
}

func (n *FCMNotifier) NotifyNewChatMessage(ctx context.Context, recipientID uuid.UUID, senderID uuid.UUID, senderName string, content string, recipientRole string) {
	if n == nil || n.client == nil || n.pool == nil {
		return
	}

	var query string
	if recipientRole == "listener" {
		query = `SELECT device_token FROM listener_android_fcm_devices WHERE listener_id = $1`
	} else {
		query = `SELECT device_token FROM user_android_fcm_devices WHERE user_id = $1`
	}

	rows, err := n.pool.Query(ctx, query, recipientID)
	if err != nil {
		log.Printf("FCM token lookup failed for chat recipient %s (%s): %v", recipientID, recipientRole, err)
		return
	}
	defer rows.Close()

	title := strings.TrimSpace(senderName)
	if title == "" {
		title = "New Message"
	}
	bodySnippet := strings.TrimSpace(content)
	if len(bodySnippet) > 120 {
		bodySnippet = bodySnippet[:117] + "..."
	}

	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			continue
		}
		_, err := n.client.Send(ctx, &messaging.Message{
			Token: token,
			Notification: &messaging.Notification{
				Title: title,
				Body:  bodySnippet,
			},
			Data: map[string]string{
				"type":        "new_chat_message",
				"partner_id":  senderID.String(),
				"sender_name": title,
				"content":     content,
			},
			Android: &messaging.AndroidConfig{
				Priority: "high",
				Notification: &messaging.AndroidNotification{
					ChannelID:             "true_line_chats",
					Sound:                 "default",
					DefaultSound:          true,
					DefaultVibrateTimings: true,
				},
			},
		})
		if err != nil {
			log.Printf("FCM chat notification failed for recipient %s: %v", recipientID, err)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("FCM token iteration failed for chat recipient %s: %v", recipientID, err)
	}
}
