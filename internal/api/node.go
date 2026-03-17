package api

import (
	"context"

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
	ident        *identity.Identity
	tp           transport.MessageTransport
	bus          event.Bus
	reachability transport.ReachabilityStatus
}

func NewNodeService(ident *identity.Identity, tp transport.MessageTransport, bus event.Bus) *NodeService {
	s := &NodeService{
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
	ch := s.bus.Subscribe(context.Background(), event.EventNodeReachability)
	for ev := range ch {
		if status, ok := ev.Data.(transport.ReachabilityStatus); ok {
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
