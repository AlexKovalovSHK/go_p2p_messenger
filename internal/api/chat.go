package api

import (
	"context"
	"fmt"
	"time"

	"github.com/user/aether/internal/storage"
	"github.com/user/aether/internal/logic"
	"github.com/user/aether/internal/transport"
	"github.com/user/aether/internal/event"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/crypto"
)

// MessageDTO is the data transfer object for UI.
type MessageDTO struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Content        string    `json:"content"` // Decrypted string for UI
	SentAt         time.Time `json:"sent_at"`
	IsOwn          bool      `json:"is_own"`
	Status         string    `json:"status"` // sent, delivered, read
}

// ConversationDTO represents a chat group/direct chat summary.
type ConversationDTO struct {
	ID          string    `json:"id"`
	LastMessage string    `json:"last_message"`
	LastSentAt  time.Time `json:"last_sent_at"`
	UnreadCount int       `json:"unread_count"`
}

// ChatService provides high-level messaging operations.
type ChatService struct {
	msgRepo   *storage.MessageRepository
	processor *logic.MessageProcessor
	tp        transport.MessageTransport
	bus       *event.Bus
}

func NewChatService(msgRepo *storage.MessageRepository, processor *logic.MessageProcessor, tp transport.MessageTransport, bus *event.Bus) *ChatService {
	return &ChatService{
		msgRepo:   msgRepo,
		processor: processor,
		tp:        tp,
		bus:       bus,
	}
}

// ListConversations returns a summary of all active chats.
func (s *ChatService) ListConversations(ctx context.Context) ([]ConversationDTO, error) {
	summaries, err := s.msgRepo.GetConversations(ctx)
	if err != nil {
		return nil, err
	}

	dtos := make([]ConversationDTO, 0, len(summaries))
	for _, sum := range summaries {
		lastMsg := string(sum.LastContent)
		if decrypted, err := s.processor.Decrypt(sum.LastContent); err == nil {
			lastMsg = string(decrypted)
		}

		dtos = append(dtos, ConversationDTO{
			ID:          sum.ConversationID,
			LastMessage: lastMsg,
			LastSentAt:  time.Unix(sum.LastSentAt, 0),
			UnreadCount: sum.UnreadCount,
		})
	}
	return dtos, nil
}

// GetMessages returns messages for a conversation with pagination.
func (s *ChatService) GetMessages(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]MessageDTO, error) {
	msgs, err := s.msgRepo.GetMessagesByChat(ctx, conversationID, limit)
	if err != nil {
		return nil, err
	}

	dtos := make([]MessageDTO, 0, len(msgs))
	for _, m := range msgs {
		content := string(m.Content)
		if decrypted, err := s.processor.Decrypt(m.Content); err == nil {
			content = string(decrypted)
		}

		dtos = append(dtos, MessageDTO{
			ID:             m.ID,
			ConversationID: m.ConversationID,
			SenderID:       m.SenderID,
			Content:        content,
			SentAt:         time.Unix(m.SentAt, 0),
			Status:         "sent", // Simple mapping for now
		})
	}
	return dtos, nil
}

// SendMessage encrypts, signs, saves, and sends a message to a recipient.
func (s *ChatService) SendMessage(ctx context.Context, recipientID peer.ID, recipientPubKey crypto.PubKey, content string) (*MessageDTO, error) {
	// 1. Seal Message
	payload := []byte(content)
	encPayload, sig, err := s.processor.SealMessage(recipientPubKey, payload)
	if err != nil {
		return nil, fmt.Errorf("seal message: %w", err)
	}

	// 2. Save to local DB
	msg := &storage.Message{
		ID:              fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		ConversationID:  recipientID.String(), // Using peerID as conversationID for direct chat
		SenderID:        "self",               // Placeholder for self ID
		Content:         encPayload,
		GlobalSeq:       0, // PN will assign global seq
		SenderSignature: sig,
		SentAt:          time.Now().Unix(),
	}

	if err := s.msgRepo.Save(ctx, msg); err != nil {
		return nil, fmt.Errorf("save message: %w", err)
	}

	// 3. Send over transport
	if err := s.tp.Send(ctx, recipientID, encPayload); err != nil {
		// Log error but the message is already in DB for retry (next sprint)
		fmt.Printf("Warning: Failed to send message to %s: %v\n", recipientID, err)
	}

	dto := &MessageDTO{
		ID:             msg.ID,
		ConversationID: msg.ConversationID,
		SenderID:       msg.SenderID,
		Content:        content,
		SentAt:         time.Unix(msg.SentAt, 0),
		IsOwn:          true,
		Status:         "sent",
	}

	// 4. Notify UI via standardized Event
	s.bus.Publish(event.TopicNewMessage, event.MessageEvent{
		ID:         dto.ID,
		ChatID:     dto.ConversationID,
		SenderID:   dto.SenderID,
		Text:       dto.Content,
		Timestamp:  dto.SentAt.Unix(),
		IsIncoming: false,
		Status:     dto.Status,
	})

	return dto, nil
}

