package sync

import (
	"context"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/user/aether/internal/event"
	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/storage"
	pb "github.com/user/aether/proto/aether"
)

// SyncEngine orchestrates the synchronization process with a Personal Node.
type SyncEngine struct {
	h        host.Host
	ident    *identity.Identity
	msgRepo  *storage.MessageRepository
	syncRepo *storage.DeviceSyncRepository
	bus      event.Bus

	nodeID   peer.ID
	nodeAddr string
}

func NewSyncEngine(h host.Host, ident *identity.Identity, msgRepo *storage.MessageRepository, syncRepo *storage.DeviceSyncRepository, bus event.Bus) *SyncEngine {
	return &SyncEngine{
		h:        h,
		ident:    ident,
		msgRepo:  msgRepo,
		syncRepo: syncRepo,
		bus:      bus,
	}
}

// SetPersonalNode updates the target PN for synchronization.
func (e *SyncEngine) SetPersonalNode(nodeID peer.ID, addr string) {
	e.nodeID = nodeID
	e.nodeAddr = addr
}

// Start runs the sync loop in the background.
func (e *SyncEngine) Start(ctx context.Context) {
	go e.run(ctx)
}

func (e *SyncEngine) run(ctx context.Context) {
	for {
		if e.nodeID == "" {
			time.Sleep(5 * time.Second)
			continue
		}

		select {
		case <-ctx.Done():
			return
		default:
			if err := e.syncCycle(ctx); err != nil {
				log.Printf("SyncEngine: cycle failed: %v, retrying in 10s...", err)
				time.Sleep(10 * time.Second)
			} else {
				// Wait before next cycle OR wait for push
				time.Sleep(30 * time.Second)
			}
		}
	}
}

func (e *SyncEngine) syncCycle(ctx context.Context) error {
	client := NewSyncClient(e.h, e.ident, e.nodeID)
	rw, err := client.Authenticate(ctx)
	if err != nil {
		return err
	}
	defer rw.Close()

	e.bus.Publish(event.Event{Type: event.EventSyncStarted})

	err = client.FetchLoop(ctx, rw, func(msgs []*pb.SyncMessage) error {
		for _, m := range msgs {
			// Save to DB
			err := e.msgRepo.Save(ctx, &storage.Message{
				ID:             m.Id,
				ConversationID: m.ConversationId,
				SenderID:       m.SenderId,
				Content:        m.Content,
				GlobalSeq:      m.GlobalSeq,
				SenderSignature: m.SenderSignature,
				SentAt:         m.SentAt,
			})
			if err != nil {
				return err
			}

			// Notify UI/others
			e.bus.Publish(event.Event{
				Type: event.EventMessageReceived,
				Data: m, // Or a DTO
			})
		}
		
		if len(msgs) > 0 {
			lastSeq := msgs[len(msgs)-1].GlobalSeq
			_ = e.syncRepo.UpdateLastSeq(ctx, e.ident.DeviceID(), lastSeq, time.Now().Unix())
		}
		
		return nil
	})

	if err == nil {
		e.bus.Publish(event.Event{Type: event.EventSyncCompleted})
	} else {
		e.bus.Publish(event.Event{Type: event.EventSyncFailed, Data: err.Error()})
	}

	return err
}
