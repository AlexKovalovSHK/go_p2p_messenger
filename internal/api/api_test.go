package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/storage"
	"github.com/user/aether/internal/logic"
	"github.com/user/aether/internal/transport"
	"github.com/user/aether/internal/event"
	"github.com/user/aether/internal/identity"
)

func TestChatService_GetMessages(t *testing.T) {
	ctx := context.Background()
	dbPath := t.TempDir() + "/test.db"
	db, _ := storage.Open(dbPath)
	storage.RunMigrations(db)
	msgRepo := storage.NewMessageRepository(db)

	// Seed data
	err := msgRepo.Save(ctx, &storage.Message{
		ID:             "msg1",
		ConversationID: "conv1",
		SenderID:       "alice",
		Content:        []byte("ENC:hello"),
		GlobalSeq:      1,
		SenderSignature: []byte("sig1"),
		SentAt:         time.Now().Unix(),
	})
	require.NoError(t, err)

	bus := event.NewBus()
	proc := logic.NewMessageProcessor(nil, bus, msgRepo)
	tp := transport.NewMockTransport()

	svc := api.NewChatService(msgRepo, proc, tp, bus)
	msgs, err := svc.GetMessages(ctx, "conv1", 0, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 1)
	assert.Equal(t, "hello", msgs[0].Content)
}

func TestNodeService_GetStatus(t *testing.T) {
	bus := event.NewBus()
	tp := transport.NewMockTransport()
	mgr := identity.NewIdentityManager(t.TempDir() + "/key")
	id, _ := mgr.Generate()

	svc := api.NewNodeService(mgr, id, tp, bus)
	status := svc.GetStatus(context.Background())

	assert.Equal(t, id.DeviceID(), status.DeviceID)
	assert.Equal(t, transport.ReachabilityUnknown, status.Reachability)
}
