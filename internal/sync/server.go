package sync

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-msgio"
	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/storage"
	pb "github.com/user/aether/proto/aether"
)

const (
	SyncProtocolID = "/aether/sync/1.0.0"
	ChallengeSize  = 32
	ChallengeTTL   = 1 * time.Minute
)

// PersonalNodeServer handles synchronization requests from linked devices.
type PersonalNodeServer struct {
	host       host.Host
	msgRepo    *storage.MessageRepository
	syncRepo   *storage.DeviceSyncRepository
	trustedIDs map[peer.ID]bool
	
	mu       sync.RWMutex
	sessions map[peer.ID]msgio.ReadWriteCloser
}

// NewPersonalNodeServer creates a new PN server instance.
func NewPersonalNodeServer(h host.Host, trusted []peer.ID, msgRepo *storage.MessageRepository, syncRepo *storage.DeviceSyncRepository) *PersonalNodeServer {
	trustedMap := make(map[peer.ID]bool)
	for _, id := range trusted {
		trustedMap[id] = true
	}

	return &PersonalNodeServer{
		host:       h,
		msgRepo:    msgRepo,
		syncRepo:   syncRepo,
		trustedIDs: trustedMap,
		sessions:   make(map[peer.ID]msgio.ReadWriteCloser),
	}
}

// Start registers the protocol handler.
func (s *PersonalNodeServer) Start() {
	s.host.SetStreamHandler(SyncProtocolID, s.HandleSyncStream)
}

// HandleSyncStream manages a long-lived synchronization session.
func (s *PersonalNodeServer) HandleSyncStream(stream network.Stream) {
	defer stream.Close()
	remotePeer := stream.Conn().RemotePeer()
	rw := msgio.NewReadWriter(stream)

	// 1. Handshake / Auth
	if !s.trustedIDs[remotePeer] {
		s.sendAuthResult(rw, false, "device not registered")
		return
	}

	if err := s.authenticate(rw, remotePeer); err != nil {
		log.Printf("PN: Auth failed for %s: %v", remotePeer, err)
		return
	}

	// Register session for Push
	s.mu.Lock()
	s.sessions[remotePeer] = rw
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.sessions, remotePeer)
		s.mu.Unlock()
	}()

	// 2. Main Loop (Fetch/Ack/Push)
	for {
		req := &pb.FetchRequest{}
		if err := ReadMsg(rw, req); err != nil {
			if err != io.EOF {
				log.Printf("PN: Failed to read request: %v", err)
			}
			break
		}

		if err := s.handleFetch(rw, remotePeer, req); err != nil {
			log.Printf("PN: Failed to handle fetch: %v", err)
			break
		}
	}
}

// PushNewMessage broadcasts a new message to the intended recipient's active devices.
func (s *PersonalNodeServer) PushNewMessage(ctx context.Context, msg *storage.Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	env := &pb.PushEnvelope{
		Type: &pb.PushEnvelope_NewMessage{
			NewMessage: &pb.SyncMessage{
				Id:              msg.ID,
				ConversationId:  msg.ConversationID,
				SenderId:        msg.SenderID,
				Content:         msg.Content,
				GlobalSeq:       msg.GlobalSeq,
				SenderSignature: msg.SenderSignature,
				SentAt:          msg.SentAt,
			},
		},
	}

	// In Aether's Personal Node model, the PN pushes to all currently online device sessions.
	// For simplicity, we push to EVERY online linked device.
	for _, rw := range s.sessions {
		_ = WriteMsg(rw, env)
	}
}

func (s *PersonalNodeServer) handleFetch(rw msgio.ReadWriteCloser, deviceID peer.ID, req *pb.FetchRequest) error {
	// 1. Query messages from DB since after_seq
	// Note: In a real app, we would query messages intended for this user/conversation.
	// For now, we fetch globally based on seq.
	limit := int(req.Limit)
	if limit == 0 || limit > 100 {
		limit = 100
	}

	msgs, err := s.msgRepo.GetSince(context.Background(), "", req.AfterSeq, limit)
	if err != nil {
		log.Printf("PN: DB error in GetSince: %v", err)
		return err
	}

	log.Printf("PN: Found %d messages since seq %d", len(msgs), req.AfterSeq)

	resp := &pb.FetchResponse{
		Messages: make([]*pb.SyncMessage, 0, len(msgs)),
		HasMore:  len(msgs) == limit,
	}

	maxSeq := req.AfterSeq
	for _, m := range msgs {
		resp.Messages = append(resp.Messages, &pb.SyncMessage{
			Id:              m.ID,
			ConversationId:  m.ConversationID,
			SenderId:        m.SenderID,
			Content:         m.Content,
			GlobalSeq:       m.GlobalSeq,
			SenderSignature: m.SenderSignature,
			SentAt:          m.SentAt,
		})
		if m.GlobalSeq > maxSeq {
			maxSeq = m.GlobalSeq
		}
	}
	resp.MaxSeq = maxSeq

	return WriteMsg(rw, resp)
}

func (s *PersonalNodeServer) authenticate(rw msgio.ReadWriteCloser, remoteID peer.ID) error {
	// Generate Challenge
	nonce := make([]byte, ChallengeSize)
	rand.Read(nonce)
	challenge := &pb.AuthChallenge{
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(ChallengeTTL).Unix(),
	}

	if err := WriteMsg(rw, challenge); err != nil {
		return err
	}

	// Read Response
	resp := &pb.AuthResponse{}
	if err := ReadMsg(rw, resp); err != nil {
		return err
	}

	// Verify Signature
	pubKey, err := remoteID.ExtractPublicKey()
	if err != nil {
		return err
	}

	valid, err := identity.Verify(pubKey, nonce, resp.Signature)
	if err != nil || !valid {
		s.sendAuthResult(rw, false, "invalid signature")
		return errors.New("auth: invalid signature")
	}

	return s.sendAuthResult(rw, true, "")
}

func (s *PersonalNodeServer) sendAuthResult(rw msgio.ReadWriteCloser, ok bool, errMsg string) error {
	res := &pb.AuthResult{
		Ok:           ok,
		ErrorMessage: errMsg,
	}
	return WriteMsg(rw, res)
}

