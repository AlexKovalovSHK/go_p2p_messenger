package transport

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

const PexProtocolID = "/aether/pex/1.0.0"

// PexHandler handles the Custom Peer Exchange functionality.
type PexHandler struct {
	host      host.Host
	isTrusted func(peer.ID) bool
}

// NewPexHandler creates a new PEX handler and registers it.
func NewPexHandler(h host.Host, isTrusted func(peer.ID) bool) *PexHandler {
	p := &PexHandler{
		host:      h,
		isTrusted: isTrusted,
	}
	h.SetStreamHandler(PexProtocolID, p.handleStream)
	return p
}

type PexResponse struct {
	Peers []peer.AddrInfo `json:"peers"`
}

func (p *PexHandler) handleStream(s network.Stream) {
	defer s.Close()

	if !p.isTrusted(s.Conn().RemotePeer()) {
		s.Reset()
		log.Printf("PEX: Rejected untrusted peer %s", s.Conn().RemotePeer())
		return
	}

	var addrs []peer.AddrInfo
	conns := p.host.Network().Conns()
	seen := make(map[peer.ID]bool)

	for _, c := range conns {
		if c.RemotePeer() == s.Conn().RemotePeer() || seen[c.RemotePeer()] {
			continue
		}
		seen[c.RemotePeer()] = true
		
		addrs = append(addrs, peer.AddrInfo{
			ID:    c.RemotePeer(),
			Addrs: p.host.Peerstore().Addrs(c.RemotePeer()),
		})
		
		if len(addrs) >= 50 { // max peers roughly
			break
		}
	}

	resp := PexResponse{Peers: addrs}
	if err := json.NewEncoder(s).Encode(resp); err != nil {
		s.Reset()
	}
}

// RequestPeers requests peers from a trusted peer.
func (p *PexHandler) RequestPeers(ctx context.Context, to peer.ID) ([]peer.AddrInfo, error) {
	s, err := p.host.NewStream(ctx, to, PexProtocolID)
	if err != nil {
		return nil, err
	}
	defer s.Close()

	s.SetReadDeadline(time.Now().Add(5 * time.Second))

	var resp PexResponse
	if err := json.NewDecoder(s).Decode(&resp); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}

	return resp.Peers, nil
}
