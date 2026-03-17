package transport

import (
	"context"

	"github.com/libp2p/go-libp2p/core/peer"
)

// ReachabilityStatus represents the node's NAT status.
type ReachabilityStatus int

const (
	ReachabilityUnknown ReachabilityStatus = iota
	ReachabilityPublic
	ReachabilityPrivate
)

// MessageTransport provides reliable P2P message delivery.
type MessageTransport interface {
	// Send sends a payload to a specific peer.
	Send(ctx context.Context, to peer.ID, payload []byte) error
	
	// Subscribe registers a handler for incoming messages.
	Subscribe(handler func(from peer.ID, payload []byte))
	
	// Start initializes and starts the transport.
	Start(ctx context.Context) error
	
	// Stop gracefully shuts down the transport.
	Stop() error
}

// PeerDiscovery provides mechanisms to find peers on the network.
type PeerDiscovery interface {
	// FindPeer attempts to locate a peer by their ID.
	FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error)
	
	// Provide announces this node to the network.
	Provide(ctx context.Context) error
}
