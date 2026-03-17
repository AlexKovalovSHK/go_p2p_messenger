package sync_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/storage"
	"github.com/user/aether/internal/sync"
	"github.com/user/aether/internal/transport"
	pb "github.com/user/aether/proto/aether"
)

func createHost(t *testing.T) (host.Host, *identity.Identity) {
	mgr := identity.NewIdentityManager(t.TempDir() + "/key")
	id, err := mgr.Generate()
	require.NoError(t, err)
	h, err := transport.NewLibp2pHost(id, 0)
	require.NoError(t, err)
	return h, id
}

func TestSync_Handshake(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. PN (Server)
	pnHost, _ := createHost(t)
	defer pnHost.Close()

	// 2. Device (Client)
	devHost, devId := createHost(t)
	defer devHost.Close()

	// PN trusts Device
	server := sync.NewPersonalNodeServer(pnHost, []peer.ID{devHost.ID()}, nil, nil)
	server.Start()

	// Connect
	err := devHost.Connect(ctx, peer.AddrInfo{ID: pnHost.ID(), Addrs: pnHost.Addrs()})
	require.NoError(t, err)

	// Handshake
	client := sync.NewSyncClient(devHost, devId, pnHost.ID())
	rw, err := client.Authenticate(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, rw)
}

func TestSync_FetchMessages(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPath := t.TempDir() + "/test.db"
	db, _ := storage.Open(dbPath)
	storage.RunMigrations(db)
	msgRepo := storage.NewMessageRepository(db)

	// Seed DB with message
	err := msgRepo.Save(ctx, &storage.Message{
		ID:             "msg1",
		ConversationID: "conv1",
		SenderID:       "alice",
		Content:        []byte("hello"),
		GlobalSeq:      1,
		SenderSignature: []byte("mock-signature"),
		SentAt:         time.Now().Unix(),
	})
	require.NoError(t, err)
	t.Log("Message seeded in DB")

	// Verify locally
	all, err := msgRepo.GetSince(ctx, "", 0, 10)
	require.NoError(t, err)
	require.Len(t, all, 1, "Message should be in DB locally")
	t.Logf("Local verification success: %d messages found", len(all))

	pnHost, _ := createHost(t)
	defer pnHost.Close()
	devHost, devId := createHost(t)
	defer devHost.Close()

	server := sync.NewPersonalNodeServer(pnHost, []peer.ID{devHost.ID()}, msgRepo, nil)
	server.Start()

	devHost.Connect(ctx, peer.AddrInfo{ID: pnHost.ID(), Addrs: pnHost.Addrs()})

	client := sync.NewSyncClient(devHost, devId, pnHost.ID())
	rw, err := client.Authenticate(ctx)
	require.NoError(t, err)
	t.Log("Handshake successful")

	received := make(chan *pb.SyncMessage, 1)
	err = client.FetchLoop(ctx, rw, func(msgs []*pb.SyncMessage) error {
		t.Logf("Received batch of %d messages", len(msgs))
		for _, m := range msgs {
			received <- m
		}
		return nil
	})
	assert.NoError(t, err)
	t.Log("FetchLoop finished")

	select {
	case m := <-received:
		t.Logf("Got message: %s", m.Id)
		assert.Equal(t, "msg1", m.Id)
		assert.Equal(t, []byte("hello"), m.Content)
	case <-time.After(5 * time.Second):
		t.Log("Timeout waiting for message in channel")
		t.Fatal("Did not receive message via sync")
	}
}

func TestSync_RealTimePush(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pnHost, _ := createHost(t)
	defer pnHost.Close()
	devHost, devId := createHost(t)
	defer devHost.Close()

	server := sync.NewPersonalNodeServer(pnHost, []peer.ID{devHost.ID()}, nil, nil)
	server.Start()

	err := devHost.Connect(ctx, peer.AddrInfo{ID: pnHost.ID(), Addrs: pnHost.Addrs()})
	require.NoError(t, err)

	client := sync.NewSyncClient(devHost, devId, pnHost.ID())
	rw, err := client.Authenticate(ctx)
	require.NoError(t, err)

	pushed := make(chan *pb.SyncMessage, 1)
	
	// Start a listener for pushes
	go func() {
		for {
			env := &pb.PushEnvelope{}
			if err := sync.ReadMsg(rw, env); err != nil {
				return
			}
			if msg := env.GetNewMessage(); msg != nil {
				pushed <- msg
			}
		}
	}()

	// Push from server
	server.PushNewMessage(ctx, &storage.Message{
		ID:              "push1",
		ConversationID:  "conv1",
		SenderID:        "alice",
		Content:         []byte("pushed"),
		GlobalSeq:       10,
		SenderSignature: []byte("sig"),
		SentAt:          time.Now().Unix(),
	})

	select {
	case m := <-pushed:
		assert.Equal(t, "push1", m.Id)
		assert.Equal(t, []byte("pushed"), m.Content)
	case <-time.After(5 * time.Second):
		t.Fatal("Did not receive pushed message")
	}
}
