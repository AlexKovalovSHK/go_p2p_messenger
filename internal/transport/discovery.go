package transport

import (
	"context"
	"fmt"
	"log"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	dual "github.com/libp2p/go-libp2p-kad-dht/dual"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	aetherEvent "github.com/user/aether/internal/event"
)

type mdnsNotifee struct {
	h       host.Host
	handler func(peer.AddrInfo)
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	if pi.ID == n.h.ID() {
		return
	}
	n.handler(pi)
}

// StartMDNS starts mDNS discovery on the local network.
func StartMDNS(h host.Host, serviceTag string, onFound func(peer.AddrInfo)) (mdns.Service, error) {
	notifee := &mdnsNotifee{h: h, handler: onFound}
	svc := mdns.NewMdnsService(h, serviceTag, notifee)
	if err := svc.Start(); err != nil {
		return nil, fmt.Errorf("start mdns: %w", err)
	}
	return svc, nil
}

// SetupAutoNAT subscribes to reachability changes and publishes them to the event bus.
func SetupAutoNAT(h host.Host, bus *aetherEvent.Bus, onReachabilityChanged func(ReachabilityStatus)) {
	sub, err := h.EventBus().Subscribe(new(event.EvtLocalReachabilityChanged))
	if err != nil {
		log.Printf("Failed to subscribe to reachability events: %v", err)
		return
	}

	go func() {
		defer sub.Close()
		for e := range sub.Out() {
			ev, ok := e.(event.EvtLocalReachabilityChanged)
			if !ok {
				continue
			}
			var status ReachabilityStatus
			switch ev.Reachability {
			case network.ReachabilityPublic:
				status = ReachabilityPublic
			case network.ReachabilityPrivate:
				status = ReachabilityPrivate
			default:
				status = ReachabilityUnknown
			}
			onReachabilityChanged(status)
			bus.Publish(aetherEvent.TopicNodeReachability, status)
		}
	}()
}

// DHTDiscovery implements PeerDiscovery using go-libp2p-kad-dht.
type DHTDiscovery struct {
	dht *dual.DHT
}

// NewDHTDiscovery creates and bootstraps a new Dual DHT.
func NewDHTDiscovery(ctx context.Context, h host.Host) (*DHTDiscovery, error) {
	kdht, err := dual.New(ctx, h, dual.DHTOption(dht.Mode(dht.ModeAutoServer)))
	if err != nil {
		return nil, fmt.Errorf("create dual dht: %w", err)
	}

	if err := kdht.Bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("bootstrap dht: %w", err)
	}

	return &DHTDiscovery{dht: kdht}, nil
}

// FindPeer attempts to locate a peer by their ID.
func (d *DHTDiscovery) FindPeer(ctx context.Context, id peer.ID) (peer.AddrInfo, error) {
	return d.dht.FindPeer(ctx, id)
}

// Provide announces this node to the network.
func (d *DHTDiscovery) Provide(ctx context.Context) error {
	return nil
}
