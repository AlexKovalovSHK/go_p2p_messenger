package storage

import (
	"context"
	"database/sql"
	"fmt"
)

// Message represents a single chat message.
type Message struct {
	ID              string
	ConversationID  string
	SenderID        string
	Content         []byte
	GlobalSeq       int64
	SenderSignature []byte
	SentAt          int64
	IsIncoming      bool
	Status          string
	DeliveredAt     *int64
	ReadAt          *int64
}

// MessageRepository handles data operations for messages.
type MessageRepository struct {
	db *DB
}

// NewMessageRepository creates a new MessageRepository.
func NewMessageRepository(db *DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Save attempts to insert a new message. Uses INSERT OR IGNORE for idempotency.
func (r *MessageRepository) Save(ctx context.Context, msg *Message) error {
	return r.db.WriteTransaction(func(tx *sql.Tx) error {
		// If global_seq is 0, we try to assign one locally to keep the UNIQUE constraint happy
		// if we strictly need it, but since we removed NOT NULL, we can just pass nil if it's 0.
		// However, for local ordering it's better to have it.
		
		var seq interface{}
		if msg.GlobalSeq > 0 {
			seq = msg.GlobalSeq
		} else {
			// Local messages get a high sequence or NULL. 
			// Let's use NULL for now to avoid collisions with PN sequences.
			seq = nil 
		}

		query := `
			INSERT OR IGNORE INTO messages 
			(id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, chat_id, timestamp, delivered_at, read_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err := tx.ExecContext(ctx, query,
			msg.ID, msg.ConversationID, msg.SenderID, msg.Content, seq, msg.SenderSignature, msg.SentAt, msg.IsIncoming, msg.Status, msg.ConversationID, msg.SentAt, msg.DeliveredAt, msg.ReadAt)
		return err
	})
}

// GetSince returns up to 'limit' messages that have global_seq > afterSeq.
// If conversationID is empty, it returns messages from all conversations.
func (r *MessageRepository) GetSince(ctx context.Context, conversationID string, afterSeq int64, limit int) ([]*Message, error) {
	var query string
	var args []interface{}

	if conversationID == "" {
		query = `
			SELECT id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, delivered_at, read_at 
			FROM messages 
			WHERE global_seq > ? 
			ORDER BY global_seq ASC 
			LIMIT ?
		`
		args = []interface{}{afterSeq, limit}
	} else {
		query = `
			SELECT id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, delivered_at, read_at 
			FROM messages 
			WHERE conversation_id = ? AND global_seq > ? 
			ORDER BY global_seq ASC 
			LIMIT ?
		`
		args = []interface{}{conversationID, afterSeq, limit}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		var seq sql.NullInt64
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &seq, &m.SenderSignature, &m.SentAt, &m.IsIncoming, &m.Status, &m.DeliveredAt, &m.ReadAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if seq.Valid {
			m.GlobalSeq = seq.Int64
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetMessagesByChat returns messages for a specific chat ordered by timestamp.
// This is used by the UI to show all messages regardless of synchronization status.
func (r *MessageRepository) GetMessagesByChat(ctx context.Context, chatID string, limit int) ([]*Message, error) {
	query := `
		SELECT id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, chat_id, timestamp, delivered_at, read_at 
		FROM messages 
		WHERE chat_id = ? 
		ORDER BY timestamp ASC 
		LIMIT ?
	`
	rows, err := r.db.QueryContext(ctx, query, chatID, limit)
	if err != nil {
		return nil, fmt.Errorf("query messages by chat: %w", err)
	}
	defer rows.Close()

	var msgs []*Message
	for rows.Next() {
		m := &Message{}
		var seq sql.NullInt64
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &seq, &m.SenderSignature, &m.SentAt, &m.IsIncoming, &m.Status, &m.ConversationID, &m.SentAt, &m.DeliveredAt, &m.ReadAt); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if seq.Valid {
			m.GlobalSeq = seq.Int64
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// ConversationSummary represents the last message and unread count for a conversation.
type ConversationSummary struct {
	ConversationID string
	LastContent    []byte
	LastSentAt     int64
	UnreadCount    int
}

// GetConversations returns a summary of all conversations.
func (r *MessageRepository) GetConversations(ctx context.Context) ([]ConversationSummary, error) {
	// Simple query to get the last message per conversation
	query := `
		SELECT conversation_id, content, sent_at, 
		       (SELECT COUNT(*) FROM messages m2 WHERE m2.conversation_id = m1.conversation_id AND m2.read_at IS NULL) as unread_count
		FROM messages m1
		WHERE id IN (SELECT MAX(id) FROM messages GROUP BY conversation_id)
		ORDER BY sent_at DESC
	`
	// Note: In a production app, we'd probably have a specific conversations table
	// and more complex unread count logic. This serves Sprint 4 requirements.
	
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []ConversationSummary
	for rows.Next() {
		s := ConversationSummary{}
		if err := rows.Scan(&s.ConversationID, &s.LastContent, &s.LastSentAt, &s.UnreadCount); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}

// Contact represents a P2P contact.
type Contact struct {
	PeerID    string
	PublicKey []byte
	Multiaddr *string
	Alias     *string
	Trusted   bool
	LastSeen  int64
	CreatedAt int64
}

// ContactRepository handles data operations for contacts.
type ContactRepository struct {
	db *DB
}

func NewContactRepository(db *DB) *ContactRepository {
	return &ContactRepository{db: db}
}

func (r *ContactRepository) Add(ctx context.Context, c *Contact) error {
	return r.db.WriteTransaction(func(tx *sql.Tx) error {
		query := `INSERT OR REPLACE INTO contacts (peer_id, public_key, multiaddr, alias, trusted, last_seen, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`
		
		var pk interface{}
		if len(c.PublicKey) > 0 {
			pk = c.PublicKey
		} else {
			pk = nil
		}

		_, err := tx.ExecContext(ctx, query, c.PeerID, pk, c.Multiaddr, c.Alias, c.Trusted, c.LastSeen, c.CreatedAt)
		return err
	})
}

func (r *ContactRepository) GetContacts(ctx context.Context) ([]*Contact, error) {
	query := `SELECT peer_id, public_key, multiaddr, alias, trusted, last_seen, created_at FROM contacts`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []*Contact
	for rows.Next() {
		c := &Contact{}
		if err := rows.Scan(&c.PeerID, &c.PublicKey, &c.Multiaddr, &c.Alias, &c.Trusted, &c.LastSeen, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		contacts = append(contacts, c)
	}
	return contacts, nil
}

// DeviceSyncState represents the sync progress for a given device.
type DeviceSyncState struct {
	DeviceID      string
	LastSyncedSeq int64
	UpdatedAt     int64
}

// DeviceSyncRepository handles recording fetch progress per device.
type DeviceSyncRepository struct {
	db *DB
}

func NewDeviceSyncRepository(db *DB) *DeviceSyncRepository {
	return &DeviceSyncRepository{db: db}
}

func (r *DeviceSyncRepository) UpdateLastSeq(ctx context.Context, deviceID string, seq, updatedAt int64) error {
	return r.db.WriteTransaction(func(tx *sql.Tx) error {
		// Ensure device exists first (foreign key constraint)
		_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO devices (device_id, public_key, is_personal_node, created_at) VALUES (?, ?, 0, ?)`, deviceID, []byte{}, updatedAt)
		if err != nil {
			return err
		}

		query := `INSERT INTO device_sync_state (device_id, last_synced_seq, updated_at) VALUES (?, ?, ?)
				  ON CONFLICT(device_id) DO UPDATE SET last_synced_seq = excluded.last_synced_seq, updated_at = excluded.updated_at`
		_, err = tx.ExecContext(ctx, query, deviceID, seq, updatedAt)
		return err
	})
}

func (r *DeviceSyncRepository) GetLastSeq(ctx context.Context, deviceID string) (int64, error) {
	var seq int64
	query := `SELECT last_synced_seq FROM device_sync_state WHERE device_id = ?`
	err := r.db.QueryRowContext(ctx, query, deviceID).Scan(&seq)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	return seq, nil
}
