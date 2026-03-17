package transport

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/user/aether/internal/identity"
)

const DirectProtocolID = "/aether/direct/1.0.0"

// Libp2pTransport implements MessageTransport using go-libp2p.
type Libp2pTransport struct {
	host    host.Host
	handler func(from peer.ID, payload []byte)
}

// NewLibp2pHost creates a new configured libp2p host.
func NewLibp2pHost(ident *identity.Identity, listenPort int) (host.Host, error) {
	connManager, err := connmgr.NewConnManager(
		100, // Lowwater
		400, // HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("create conn manager: %w", err)
	}

	opts := []libp2p.Option{
		libp2p.Identity(ident.PrivateKey),
		libp2p.ListenAddrStrings(
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic-v1", listenPort),
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort),
		),
		libp2p.DefaultSecurity, // Noise
		libp2p.DefaultMuxers,   // Yamux
		libp2p.ConnectionManager(connManager),
		libp2p.NATPortMap(), // UPnP/NAT-PMP
		libp2p.EnableHolePunching(), // DCUTR
	}

	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, fmt.Errorf("create libp2p host: %w", err)
	}

	return h, nil
}

// NewLibp2pTransport creates a new Libp2pTransport using the given host.
func NewLibp2pTransport(h host.Host) *Libp2pTransport {
	t := &Libp2pTransport{
		host: h,
	}
	h.SetStreamHandler(DirectProtocolID, t.handleIncomingStream)
	return t
}

func (t *Libp2pTransport) handleIncomingStream(s network.Stream) {
	defer s.Close()
	
	data, err := io.ReadAll(s)
	if err != nil {
		return // Ignore stream resets or read errors
	}

	if t.handler != nil && len(data) > 0 {
		t.handler(s.Conn().RemotePeer(), data)
	}
}

// Send sends a payload to a specific peer by opening a new stream.
func (t *Libp2pTransport) Send(ctx context.Context, to peer.ID, payload []byte) error {
	s, err := t.host.NewStream(ctx, to, DirectProtocolID)
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer s.Close() // Closes for writing, EOF sent

	if _, err := s.Write(payload); err != nil {
		s.Reset()
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

// Subscribe registers a handler for incoming messages.
func (t *Libp2pTransport) Subscribe(handler func(from peer.ID, payload []byte)) {
	t.handler = handler
}

// Start prepares the transport. Libp2p host runs on creation.
func (t *Libp2pTransport) Start(ctx context.Context) error {
	return nil
}

// Connect attempts to establish a connection.
func (t *Libp2pTransport) Connect(ctx context.Context, to peer.ID) error {
	return t.host.Connect(ctx, peer.AddrInfo{ID: to})
}

// Stop gracefully shuts down the transport.
func (t *Libp2pTransport) Stop() error {
	return t.host.Close()
}
