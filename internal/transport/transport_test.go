package transport_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/user/aether/internal/identity"
	"github.com/user/aether/internal/transport"
	aetherEvent "github.com/user/aether/internal/event"
)

func newTestIdentity(t *testing.T) *identity.Identity {
	mgr := identity.NewIdentityManager(t.TempDir() + "/key")
	id, err := mgr.Generate()
	require.NoError(t, err)
	return id
}

func TestTransport_MDNSDiscovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hostA, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostA.Close()

	hostB, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostB.Close()

	discovered := make(chan peer.AddrInfo, 1)

	_, err = transport.StartMDNS(hostA, "aether-test", func(pi peer.AddrInfo) {
		if pi.ID == hostB.ID() {
			discovered <- pi
		}
	})
	require.NoError(t, err)

	_, err = transport.StartMDNS(hostB, "aether-test", func(pi peer.AddrInfo) {})
	require.NoError(t, err)

	select {
	case pi := <-discovered:
		assert.Equal(t, hostB.ID(), pi.ID)
	case <-ctx.Done():
		t.Fatal("mDNS: peer not discovered in 10s")
	}
}

func TestTransport_SendSubscribe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	hostA, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostA.Close()
	tPortA := transport.NewLibp2pTransport(hostA)

	hostB, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostB.Close()
	tPortB := transport.NewLibp2pTransport(hostB)

	err = hostA.Connect(ctx, peer.AddrInfo{
		ID:    hostB.ID(),
		Addrs: hostB.Addrs(),
	})
	require.NoError(t, err)

	received := make(chan []byte, 1)
	tPortB.Subscribe(func(from peer.ID, payload []byte) {
		if from == hostA.ID() {
			received <- payload
		}
	})

	err = tPortA.Send(ctx, hostB.ID(), []byte("hello"))
	require.NoError(t, err)

	select {
	case msg := <-received:
		assert.Equal(t, []byte("hello"), msg)
	case <-ctx.Done():
		t.Fatal("Message not received via transport")
	}
}

func TestTransport_QUICConnection(t *testing.T) {
	ctx := context.Background()
	hostA, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostA.Close()

	hostB, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostB.Close()

	err = hostA.Connect(ctx, peer.AddrInfo{
		ID:    hostB.ID(),
		Addrs: hostB.Addrs(),
	})
	require.NoError(t, err)

	conns := hostA.Network().ConnsToPeer(hostB.ID())
	assert.Greater(t, len(conns), 0)
}

func TestTransport_AutoNATStatus(t *testing.T) {
	hostA, err := transport.NewLibp2pHost(newTestIdentity(t), 0)
	require.NoError(t, err)
	defer hostA.Close()

	reached := make(chan transport.ReachabilityStatus, 1)
	bus := aetherEvent.NewBus()
	transport.SetupAutoNAT(hostA, bus, func(status transport.ReachabilityStatus) {
		select {
		case reached <- status:
		default:
		}
	})

	emitter, err := hostA.EventBus().Emitter(new(event.EvtLocalReachabilityChanged))
	require.NoError(t, err)
	defer emitter.Close()

	emitter.Emit(event.EvtLocalReachabilityChanged{Reachability: network.ReachabilityPrivate})

	select {
	case st := <-reached:
		assert.Equal(t, transport.ReachabilityPrivate, st)
	case <-time.After(2 * time.Second):
		t.Fatal("Did not receive AutoNAT status")
	}
}

func TestTransport_MockTransport(t *testing.T) {
	mt := transport.NewMockTransport()
	ctx := context.Background()

	id := peer.ID("peerA")
	mt.Send(ctx, id, []byte("mocked"))

	assert.Len(t, mt.SentMessages, 1)
	assert.Equal(t, id, mt.SentMessages[0].To)
	assert.Equal(t, []byte("mocked"), mt.SentMessages[0].Payload)
}

func TestTransport_ConnectUnknown(t *testing.T) {
	ctx := context.Background()
	hostA, _ := transport.NewLibp2pHost(newTestIdentity(t), 0)
	defer hostA.Close()

	addr, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/55555")
	err := hostA.Connect(ctx, peer.AddrInfo{
		ID:    peer.ID("someId"),
		Addrs: []multiaddr.Multiaddr{addr},
	})
	assert.Error(t, err)
}

func TestTransport_PEX(t *testing.T) {
	ctx := context.Background()

	hostA, _ := transport.NewLibp2pHost(newTestIdentity(t), 0)
	defer hostA.Close()

	hostB, _ := transport.NewLibp2pHost(newTestIdentity(t), 0)
	defer hostB.Close()

	hostC, _ := transport.NewLibp2pHost(newTestIdentity(t), 0)
	defer hostC.Close()

	hostA.Connect(ctx, peer.AddrInfo{ID: hostB.ID(), Addrs: hostB.Addrs()})
	hostA.Connect(ctx, peer.AddrInfo{ID: hostC.ID(), Addrs: hostC.Addrs()})

	transport.NewPexHandler(hostA, func(id peer.ID) bool {
		return id == hostB.ID()
	})

	pexB := transport.NewPexHandler(hostB, func(peer.ID) bool { return true })
	peers, err := pexB.RequestPeers(ctx, hostA.ID())
	require.NoError(t, err)
	assert.NotEmpty(t, peers)

	foundC := false
	for _, p := range peers {
		if p.ID == hostC.ID() {
			foundC = true
		}
	}
	assert.True(t, foundC, "C should be in peers")

	pexC := transport.NewPexHandler(hostC, func(peer.ID) bool { return true })
	_, err = pexC.RequestPeers(ctx, hostA.ID())
	assert.Error(t, err, "Should get stream reset for untrusted peer")
}
