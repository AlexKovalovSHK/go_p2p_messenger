package logic_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/aether/internal/event"
	"github.com/user/aether/internal/logic"
	"github.com/user/aether/internal/storage"
)

func TestMessageProcessor_ProcessIncoming(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/test_proc.db"
	db, err := storage.Open(dbPath)
	require.NoError(t, err)
	defer db.Close()
	storage.RunMigrations(db)

	msgRepo := storage.NewMessageRepository(db)
	bus := event.NewBus()
	proc := logic.NewMessageProcessor(nil, bus, msgRepo)

	// Subscribe to verify event
	sub := bus.Subscribe(event.TopicNewMessage)

	from := peer.ID("test-peer-id")
	payload := []byte("ENC:hello from peer") // Mock encryption prefix

	err = proc.ProcessIncoming(ctx, from, payload)
	assert.NoError(t, err)

	// Check DB
	msgs, err := msgRepo.GetMessagesByChat(ctx, from.String(), 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "hello from peer", string(msgs[0].Content))
	assert.True(t, msgs[0].IsIncoming)

	// Check Event
	select {
	case ev := <-sub:
		msgEv := ev.(event.MessageEvent)
		assert.Equal(t, "hello from peer", msgEv.Text)
		assert.Equal(t, from.String(), msgEv.SenderID)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for TopicNewMessage event")
	}
}
