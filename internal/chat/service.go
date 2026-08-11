package chat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"trueline-backend/internal/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatService struct {
	pool *pgxpool.Pool
}

func NewChatService(pool *pgxpool.Pool) *ChatService {
	return &ChatService{pool: pool}
}

type SendMessagePayload struct {
	Content string `json:"content"`
}

func (s *ChatService) ListUserConversations(ctx context.Context, userID uuid.UUID) ([]db.ConversationSummary, error) {
	if s.pool == nil {
		// Mock Conversation List for local app testing
		return []db.ConversationSummary{
			{
				PartnerID:           uuid.MustParse("a0000000-0000-0000-0000-000000000001"),
				PartnerName:         "Afreen",
				PartnerTitle:        "Joy Helper",
				PartnerPhotoURL:     "https://images.unsplash.com/photo-1534528741775-53994a69daeb?auto=format&fit=crop&w=400&q=80",
				PartnerAvailability: "online",
				LastMessage:         "Haan bilkul! Feel free to call anytime.",
				LastMessageSender:   "partner",
				LastMessageTime:     time.Now().Add(-15 * time.Minute),
				UnreadCount:         0,
			},
			{
				PartnerID:           uuid.MustParse("a0000000-0000-0000-0000-000000000002"),
				PartnerName:         "Ahmedi",
				PartnerTitle:        "Calm Friend",
				PartnerPhotoURL:     "https://images.unsplash.com/photo-1517841905240-472988babdf9?auto=format&fit=crop&w=400&q=80",
				PartnerAvailability: "online",
				LastMessage:         "Hello! Hope you had a peaceful day.",
				LastMessageSender:   "partner",
				LastMessageTime:     time.Now().Add(-30 * time.Minute),
				UnreadCount:         1,
			},
		}, nil
	}

	query := `
		WITH latest_messages AS (
			SELECT DISTINCT ON (partner_id)
				partner_id,
				content as last_message,
				sender_type as last_message_sender,
				created_at as last_message_time
			FROM chat_messages
			WHERE user_id = $1
			ORDER BY partner_id, created_at DESC
		),
		unread_counts AS (
			SELECT partner_id, COUNT(*) as unread_count
			FROM chat_messages
			WHERE user_id = $1 AND sender_type = 'partner' AND read_at IS NULL
			GROUP BY partner_id
		)
		SELECT 
			p.id as partner_id,
			p.name as partner_name,
			p.title as partner_title,
			p.photo_url as partner_photo_url,
			p.availability as partner_availability,
			COALESCE(lm.last_message, '') as last_message,
			COALESCE(lm.last_message_sender, '') as last_message_sender,
			COALESCE(lm.last_message_time, p.created_at) as last_message_time,
			COALESCE(uc.unread_count, 0) as unread_count
		FROM latest_messages lm
		JOIN partners p ON p.id = lm.partner_id
		LEFT JOIN unread_counts uc ON uc.partner_id = p.id
		ORDER BY lm.last_message_time DESC
	`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]db.ConversationSummary, 0)
	for rows.Next() {
		var c db.ConversationSummary
		err := rows.Scan(
			&c.PartnerID, &c.PartnerName, &c.PartnerTitle, &c.PartnerPhotoURL,
			&c.PartnerAvailability, &c.LastMessage, &c.LastMessageSender,
			&c.LastMessageTime, &c.UnreadCount,
		)
		if err != nil {
			return nil, err
		}
		conversations = append(conversations, c)
	}

	return conversations, nil
}

func (s *ChatService) GetChatMessages(ctx context.Context, userID, partnerID uuid.UUID, limit, offset int) ([]db.ChatMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	if s.pool == nil {
		now := time.Now()
		return []db.ChatMessage{
			{
				ID:         uuid.New(),
				UserID:     userID,
				PartnerID:  partnerID,
				SenderType: "user",
				Content:    "Namaste! Are you free to talk today?",
				CreatedAt:  now.Add(-2 * time.Hour),
			},
			{
				ID:         uuid.New(),
				UserID:     userID,
				PartnerID:  partnerID,
				SenderType: "partner",
				Content:    "Haan bilkul! Feel free to call or text anytime.",
				CreatedAt:  now.Add(-1 * time.Hour),
			},
		}, nil
	}

	// Automatically mark messages from partner as read
	markReadQuery := `
		UPDATE chat_messages
		SET read_at = NOW()
		WHERE user_id = $1 AND partner_id = $2 AND sender_type = 'partner' AND read_at IS NULL
	`
	_, _ = s.pool.Exec(ctx, markReadQuery, userID, partnerID)

	query := `
		SELECT id, user_id, partner_id, sender_type, content, read_at, created_at
		FROM chat_messages
		WHERE user_id = $1 AND partner_id = $2
		ORDER BY created_at ASC
		LIMIT $3 OFFSET $4
	`

	rows, err := s.pool.Query(ctx, query, userID, partnerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chat messages: %w", err)
	}
	defer rows.Close()

	messages := make([]db.ChatMessage, 0)
	for rows.Next() {
		var m db.ChatMessage
		err := rows.Scan(
			&m.ID, &m.UserID, &m.PartnerID, &m.SenderType,
			&m.Content, &m.ReadAt, &m.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}

	return messages, nil
}

func (s *ChatService) SendMessage(ctx context.Context, userID, partnerID uuid.UUID, content string) (*db.ChatMessage, error) {
	if content == "" {
		return nil, errors.New("message content cannot be empty")
	}

	if s.pool == nil {
		return &db.ChatMessage{
			ID:         uuid.New(),
			UserID:     userID,
			PartnerID:  partnerID,
			SenderType: "user",
			Content:    content,
			CreatedAt:  time.Now(),
		}, nil
	}

	var msg db.ChatMessage
	query := `
		INSERT INTO chat_messages (user_id, partner_id, sender_type, content)
		VALUES ($1, $2, 'user', $3)
		RETURNING id, user_id, partner_id, sender_type, content, read_at, created_at
	`
	err := s.pool.QueryRow(ctx, query, userID, partnerID, content).Scan(
		&msg.ID, &msg.UserID, &msg.PartnerID, &msg.SenderType,
		&msg.Content, &msg.ReadAt, &msg.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send chat message: %w", err)
	}

	return &msg, nil
}

func (s *ChatService) MarkMessagesRead(ctx context.Context, userID, partnerID uuid.UUID) error {
	if s.pool == nil {
		return nil
	}
	query := `
		UPDATE chat_messages
		SET read_at = NOW()
		WHERE user_id = $1 AND partner_id = $2 AND sender_type = 'partner' AND read_at IS NULL
	`
	_, err := s.pool.Exec(ctx, query, userID, partnerID)
	return err
}
