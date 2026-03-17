package sync

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-msgio"
	"github.com/user/aether/internal/identity"
	pb "github.com/user/aether/proto/aether"
)

// SyncClient handles authentication and message synchronization with a Personal Node.
type SyncClient struct {
	host     host.Host
	ident    *identity.Identity
	nodeID   peer.ID
	nodeAddr string
}

// NewSyncClient creates a new client for a specific Personal Node.
func NewSyncClient(h host.Host, ident *identity.Identity, nodeID peer.ID) *SyncClient {
	return &SyncClient{
		host:   h,
		ident:  ident,
		nodeID: nodeID,
	}
}

// Authenticate performs the handshake with the Personal Node and returns an active stream.
func (c *SyncClient) Authenticate(ctx context.Context) (msgio.ReadWriteCloser, error) {
	s, err := c.host.NewStream(ctx, c.nodeID, SyncProtocolID)
	if err != nil {
		return nil, fmt.Errorf("open sync stream: %w", err)
	}

	rw := msgio.NewReadWriter(s)

	// 1. Read Challenge
	challenge := &pb.AuthChallenge{}
	if err := ReadMsg(rw, challenge); err != nil {
		s.Reset()
		return nil, fmt.Errorf("read challenge: %w", err)
	}

	if time.Now().Unix() > challenge.ExpiresAt {
		s.Reset()
		return nil, fmt.Errorf("challenge expired")
	}

	// 2. Sign and respond
	sig, err := c.ident.Sign(challenge.Nonce)
	if err != nil {
		s.Reset()
		return nil, fmt.Errorf("sign nonce: %w", err)
	}

	resp := &pb.AuthResponse{
		DeviceId:  []byte(c.ident.DeviceID()),
		Signature: sig,
		Timestamp: time.Now().Unix(),
	}

	if err := WriteMsg(rw, resp); err != nil {
		s.Reset()
		return nil, fmt.Errorf("write response: %w", err)
	}

	// 3. Read Result
	result := &pb.AuthResult{}
	if err := ReadMsg(rw, result); err != nil {
		s.Reset()
		return nil, fmt.Errorf("read result: %w", err)
	}

	if !result.Ok {
		s.Reset()
		return nil, fmt.Errorf("auth rejected: %s", result.ErrorMessage)
	}

	return rw, nil
}

// FetchLoop performs the sync cycle: Fetch messages, save them, and ACK.
func (c *SyncClient) FetchLoop(ctx context.Context, rw msgio.ReadWriteCloser, saveBatch func([]*pb.SyncMessage) error) error {
	for {
		// 1. Get last synced sequence from DB (Placeholder)
		lastSeq := int64(0)
		_ = lastSeq

		// For now, let's assume we pass the start sequence or have it.
		// Let's modify the signature or assume 0 for start.
		
		req := &pb.FetchRequest{
			AfterSeq: 0, // Should be lastSeq from DB
			Limit:    100,
		}

		if err := WriteMsg(rw, req); err != nil {
			return err
		}

		resp := &pb.FetchResponse{}
		if err := ReadMsg(rw, resp); err != nil {
			return err
		}

		if len(resp.Messages) > 0 {
			if err := saveBatch(resp.Messages); err != nil {
				return fmt.Errorf("save batch: %w", err)
			}
			
			// ACK back
			ack := &pb.AckRequest{AckedSeq: resp.MaxSeq}
			if err := WriteMsg(rw, ack); err != nil {
				return err
			}
		}

		if !resp.HasMore {
			break
		}
	}
	return nil
}
