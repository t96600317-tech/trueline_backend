package chat

import (
	"context"
	"errors"
	"fmt"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatService struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func NewChatService(pool *pgxpool.Pool) *ChatService {
	return &ChatService{
		pool:    pool,
		queries: db.New(pool),
	}
}

type SendMessagePayload struct {
	Content string `json:"content"`
}

func (s *ChatService) ListConversations(ctx context.Context, actorID uuid.UUID, role string) ([]db.ConversationSummary, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	var query string
	if role == "user" {
		query = `
			WITH latest_messages AS (
				SELECT DISTINCT ON (listener_id)
					listener_id,
					content as last_message,
					sender_type as last_message_sender,
					created_at as last_message_time
				FROM chat_messages
				WHERE user_id = $1
				ORDER BY listener_id, created_at DESC
			),
			unread_counts AS (
				SELECT listener_id, COUNT(*) as unread_count
				FROM chat_messages
				WHERE user_id = $1 AND sender_type = 'listener' AND read_at IS NULL
				GROUP BY listener_id
			)
			SELECT
				l.id as listener_id,
				l.name as listener_name,
				l.title as listener_title,
				l.photo_url as listener_photo_url,
				l.availability as listener_availability,
				COALESCE(lm.last_message, '') as last_message,
				COALESCE(lm.last_message_sender, '') as last_message_sender,
				COALESCE(lm.last_message_time, l.created_at) as last_message_time,
				COALESCE(uc.unread_count, 0) as unread_count,
				false as is_regular
			FROM latest_messages lm
			JOIN listeners l ON l.id = lm.listener_id
			LEFT JOIN unread_counts uc ON uc.listener_id = l.id
			ORDER BY lm.last_message_time DESC
		`
	} else {
		query = `
			WITH latest_messages AS (
				SELECT DISTINCT ON (user_id)
					user_id,
					content as last_message,
					sender_type as last_message_sender,
					created_at as last_message_time
				FROM chat_messages
				WHERE listener_id = $1
				ORDER BY user_id, created_at DESC
			),
			unread_counts AS (
				SELECT user_id, COUNT(*) as unread_count
				FROM chat_messages
				WHERE listener_id = $1 AND sender_type = 'user' AND read_at IS NULL
				GROUP BY user_id
			),
			msg_counts AS (
				SELECT user_id, COUNT(*) as total_messages
				FROM chat_messages
				WHERE listener_id = $1
				GROUP BY user_id
			),
			call_counts AS (
				SELECT user_id, COUNT(*) as total_calls
				FROM call_sessions
				WHERE listener_id = $1 AND status IN ('ended', 'completed', 'active')
				GROUP BY user_id
			)
			SELECT
				u.id as user_id,
				COALESCE(NULLIF(u.name, ''), 'user' || (100000 + (abs(hashtext(u.id::text)) % 900000))::text) as user_name,
				'Caller' as user_title,
				'' as user_photo_url,
				CASE 
					WHEN EXISTS (
						SELECT 1 FROM chat_messages cm 
						WHERE cm.user_id = u.id AND cm.created_at > NOW() - INTERVAL '15 minutes'
					) OR EXISTS (
						SELECT 1 FROM call_sessions cs 
						WHERE cs.user_id = u.id AND (cs.status = 'active' OR cs.created_at > NOW() - INTERVAL '15 minutes')
					) OR u.updated_at > NOW() - INTERVAL '15 minutes' THEN 'online'
					ELSE 'offline'
				END as user_availability,
				COALESCE(lm.last_message, '') as last_message,
				COALESCE(lm.last_message_sender, '') as last_message_sender,
				COALESCE(lm.last_message_time, u.created_at) as last_message_time,
				COALESCE(uc.unread_count, 0) as unread_count,
				COALESCE(mc.total_messages >= 4, false) OR COALESCE(cc.total_calls >= 2, false) as is_regular
			FROM latest_messages lm
			JOIN users u ON u.id = lm.user_id
			LEFT JOIN unread_counts uc ON uc.user_id = u.id
			LEFT JOIN msg_counts mc ON mc.user_id = u.id
			LEFT JOIN call_counts cc ON cc.user_id = u.id
			ORDER BY lm.last_message_time DESC
		`
	}

	rows, err := s.pool.Query(ctx, query, actorID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]db.ConversationSummary, 0)
	for rows.Next() {
		var c db.ConversationSummary
		var targetID uuid.UUID
		var targetName, targetTitle, targetPhoto, targetAvailability string
		err := rows.Scan(
			&targetID, &targetName, &targetTitle, &targetPhoto,
			&targetAvailability, &c.LastMessage, &c.LastMessageSender,
			&c.LastMessageTime, &c.UnreadCount, &c.IsRegular,
		)
		if err != nil {
			return nil, err
		}
		c.PartnerID = targetID
		c.PartnerName = targetName
		c.PartnerTitle = targetTitle
		c.PartnerPhotoURL = targetPhoto
		c.PartnerAvailability = targetAvailability

		if role == "user" {
			c.ListenerID = targetID
			c.ListenerName = targetName
			c.ListenerTitle = targetTitle
			c.ListenerPhotoURL = targetPhoto
			c.ListenerAvailability = targetAvailability
		} else {
			c.UserID = &targetID
			c.UserName = targetName
			c.UserTitle = targetTitle
			c.UserPhotoURL = targetPhoto
			c.UserAvailability = targetAvailability
		}
		conversations = append(conversations, c)
	}

	return conversations, nil
}

func (s *ChatService) GetChatMessages(ctx context.Context, userID, listenerID uuid.UUID, role string) ([]db.ChatMessage, error) {
	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	var markReadQuery string
	if role == "user" {
		markReadQuery = `UPDATE chat_messages SET read_at = NOW() WHERE user_id = $1 AND listener_id = $2 AND sender_type = 'listener' AND read_at IS NULL`
	} else {
		markReadQuery = `UPDATE chat_messages SET read_at = NOW() WHERE user_id = $1 AND listener_id = $2 AND sender_type = 'user' AND read_at IS NULL`
	}
	_, _ = s.pool.Exec(ctx, markReadQuery, userID, listenerID)

	query := `
		SELECT id, user_id, listener_id, sender_type, content, moderation_status, read_at, created_at
		FROM chat_messages
		WHERE user_id = $1 AND listener_id = $2
		ORDER BY created_at ASC
	`

	rows, err := s.pool.Query(ctx, query, userID, listenerID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat messages: %w", err)
	}
	defer rows.Close()

	messages := make([]db.ChatMessage, 0)
	for rows.Next() {
		var m db.ChatMessage
		err := rows.Scan(
			&m.ID, &m.UserID, &m.ListenerID, &m.SenderType,
			&m.Content, &m.ModerationStatus, &m.ReadAt, &m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		m.PartnerID = m.ListenerID
		messages = append(messages, m)
	}

	return messages, nil
}

func (s *ChatService) SendMessage(ctx context.Context, userID, listenerID uuid.UUID, role, content string) (*db.ChatMessage, error) {
	if content == "" {
		return nil, errors.New("message content cannot be empty")
	}

	if s.pool == nil {
		return nil, errors.New("database not connected")
	}

	var msg db.ChatMessage
	query := `
		INSERT INTO chat_messages (user_id, listener_id, sender_type, content)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, listener_id, sender_type, content, moderation_status, read_at, created_at
	`
	err := s.pool.QueryRow(ctx, query, userID, listenerID, role, content).Scan(
		&msg.ID, &msg.UserID, &msg.ListenerID, &msg.SenderType,
		&msg.Content, &msg.ModerationStatus, &msg.ReadAt, &msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat message: %w", err)
	}

	msg.PartnerID = msg.ListenerID
	return &msg, nil
}
