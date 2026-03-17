package api

import (
	"context"
	"fmt"

	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/transport"
	"github.com/user/aether/internal/event"
)

// NodeStatus represents the current state of the P2P node.
type NodeStatus struct {
	DeviceID     string                     `json:"device_id"`
	Reachability transport.ReachabilityStatus `json:"reachability"`
	PeerCount    int                        `json:"peer_count"`
}

// NodeService provides operations for node management.
type NodeService struct {
	idMgr        identity.IdentityManager
	ident        *identity.Identity
	tp           transport.MessageTransport
	bus          *event.Bus
	reachability transport.ReachabilityStatus
}

func NewNodeService(idMgr identity.IdentityManager, ident *identity.Identity, tp transport.MessageTransport, bus *event.Bus) *NodeService {
	s := &NodeService{
		idMgr:        idMgr,
		ident:        ident,
		tp:           tp,
		bus:          bus,
		reachability: transport.ReachabilityUnknown,
	}
	
	// Start a background loop to listen for reachability events
	go s.watchReachability()
	
	return s
}

func (s *NodeService) watchReachability() {
	// Simple subscription for status
	ch := s.bus.Subscribe(event.TopicNodeReachability)
	for ev := range ch {
		if status, ok := ev.(transport.ReachabilityStatus); ok {
			s.reachability = status
		}
	}
}

// GetStatus returns the current node health and identity information.
func (s *NodeService) GetStatus(ctx context.Context) *NodeStatus {
	return &NodeStatus{
		DeviceID:     s.ident.DeviceID(),
		Reachability: s.reachability,
		PeerCount:    0, // Peer count tracking can be added if needed
	}
}

// ExportIdentity returns the private key bytes.
func (s *NodeService) ExportIdentity() ([]byte, error) {
	return identity.MarshalPrivateKey(s.ident.PrivateKey)
}

// ImportIdentity replaces the current identity with one from bytes.
func (s *NodeService) ImportIdentity(keyBytes []byte) error {
	// This would require saving to file and reloading
	// For now, let's assume we just want to update the in-memory state and the file if possible.
	// Actually, IdentityManager should handle this.
	return fmt.Errorf("not implemented natively in NodeService yet")
}

// GenerateNewIdentity creates a new identity and replaces the current one.
func (s *NodeService) GenerateNewIdentity() (*NodeStatus, error) {
	newId, err := s.idMgr.Generate()
	if err != nil {
		return nil, err
	}
	s.ident = newId
	return s.GetStatus(context.Background()), nil
}

// SetPersonalNode updates the personal node connection info.
func (s *NodeService) SetPersonalNode(ctx context.Context, addr string) error {
	// This will be implemented in Sync Sprint
	return nil
}
