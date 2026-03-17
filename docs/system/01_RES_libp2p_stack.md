# 01_RES_libp2p_stack.md — Исследование P2P-стека для Aether

**Статус:** Research / Draft  
**Дата:** 2026-03-17  
**Версия go-libp2p:** v0.38.x

---

## Оглавление

1. [Транспорты: QUIC + TCP при жёстком NAT](#1-транспорты-quic--tcp-при-жёстком-nat)
2. [Discovery: Kademlia DHT + AutoNAT](#2-discovery-kademlia-dht--autonat)
3. [PEX (Peer Exchange)](#3-pex-peer-exchange)
4. [Relay: Circuit v2](#4-relay-circuit-v2)
5. [Архитектурная сводка и риски](#5-архитектурная-сводка-и-риски)

---

## 1. Транспорты: QUIC + TCP при жёстком NAT

### 1.1 Сравнение транспортов

| Характеристика | QUIC (UDP) | TCP |
|---|---|---|
| Handshake latency | 0-RTT / 1-RTT | 3-way + TLS |
| Multiplexing | Нативный (no HoL blocking) | Через yamux/mplex |
| NAT traversal | UDP hole punching | Ненадёжен |
| Connection migration | ✅ (смена IP без реконнекта) | ❌ |

QUIC — основной транспорт. TCP — fallback для сетей, блокирующих UDP.

### 1.2 Конфигурация хоста

```go
package node

import (
    "context"
    "fmt"
    "time"

    "github.com/libp2p/go-libp2p"
    "github.com/libp2p/go-libp2p/core/crypto"
    "github.com/libp2p/go-libp2p/core/host"
    tcptransport "github.com/libp2p/go-libp2p/p2p/transport/tcp"
    libp2pquic "github.com/libp2p/go-libp2p/p2p/transport/quic"
    "github.com/libp2p/go-libp2p/p2p/transport/websocket"
    "github.com/libp2p/go-libp2p/p2p/net/connmgr"
    "github.com/libp2p/go-libp2p/p2p/security/noise"
    "github.com/libp2p/go-libp2p/p2p/muxer/yamux"
)

type Config struct {
    PrivateKey   crypto.PrivKey
    EnableRelay  bool
}

func NewHost(ctx context.Context, cfg Config) (host.Host, error) {
    connMgr, err := connmgr.NewConnManager(60, 100,
        connmgr.WithGracePeriod(30*time.Second),
    )
    if err != nil {
        return nil, fmt.Errorf("connmgr: %w", err)
    }

    opts := []libp2p.Option{
        libp2p.Identity(cfg.PrivateKey),
        libp2p.Transport(libp2pquic.NewTransport),        // QUIC — приоритет
        libp2p.Transport(tcptransport.NewTCPTransport),   // TCP — fallback
        libp2p.Transport(websocket.New),                   // WS — corporate proxy
        libp2p.ListenAddrStrings(
            "/ip4/0.0.0.0/udp/0/quic-v1",
            "/ip4/0.0.0.0/tcp/0",
            "/ip6/::/udp/0/quic-v1",
            "/ip6/::/tcp/0",
        ),
        libp2p.Security(noise.ID, noise.New),             // Noise (Ed25519)
        libp2p.Muxer("/yamux/1.0.0", yamux.DefaultTransport),
        libp2p.NATPortMap(),                               // UPnP / NAT-PMP
        libp2p.ConnectionManager(connMgr),
        libp2p.EnableHolePunching(),                       // DCUTR
    }

    if cfg.EnableRelay {
        opts = append(opts,
            libp2p.EnableAutoRelayWithStaticRelays(getBootstrapRelays()),
        )
    }

    return libp2p.New(opts...)
}
```

### 1.3 NAT Traversal — механика hole punching (DCUTR)

```
Alice (NAT)          Relay              Bob (NAT)
   |── connect ──────►|                   |
   |                   |◄─── connect ─────|
   |◄── sync(Bob) ─────|                  |
   |                   |──── sync(Alice)──►|
   |◄══════ Direct QUIC holepunch ════════►|
         (simultaneous UDP send)
```

**Важно:** Hole punching надёжен при Full Cone NAT и Address-Restricted NAT. При **Symmetric NAT** (~30% пользователей, мобильные операторы, корпоративные сети) — вероятность успеха ~30%, нужен Relay.

### 1.4 Multiaddress форматы в Aether

```
# Прямой QUIC
/ip4/203.0.113.1/udp/4001/quic-v1/p2p/12D3KooWExampleID

# Через relay при Symmetric NAT
/ip4/relay.aether.io/tcp/443/wss/p2p/RelayID/p2p-circuit/p2p/12D3KooWExampleID
```

### 1.5 Риски транспортного уровня

| Риск | Вероятность | Митигация |
|---|---|---|
| ISP блокирует UDP | Средняя | TCP fallback + WebSocket через 443 |
| Symmetric NAT | Высокая | Circuit Relay v2 |
| IPv6 недоступен | Высокая | Prefer IPv4 в DHT routing |

---

## 2. Discovery: Kademlia DHT + AutoNAT

### 2.1 DHT Server vs Client mode

```go
package discovery

import (
    "context"
    "time"

    "github.com/libp2p/go-libp2p/core/host"
    dht "github.com/libp2p/go-libp2p-kad-dht"
    "github.com/libp2p/go-libp2p-kad-dht/dual"
)

// InitDHT инициализирует Kademlia DHT.
//
// Server mode:
//   - Участвует в маршрутизации, анонсирует других
//   - Подходит для Personal Node (всегда онлайн)
//   - Требует публичный IP или успешный AutoNAT
//
// Client mode:
//   - Только запросы, не маршрутизирует
//   - Для мобильных / NAT-узлов (~70% экономии трафика)
func InitDHT(ctx context.Context, h host.Host, isServerNode bool) (*dual.DHT, error) {
    mode := dht.ModeClient
    if isServerNode {
        mode = dht.ModeServer
    }

    d, err := dual.New(ctx, h,
        dual.WanDHTOption(dht.Mode(mode)),
        dual.WanDHTOption(dht.BootstrapPeers(dht.GetDefaultBootstrapPeerAddrInfos()...)),
        dual.WanDHTOption(dht.BucketSize(20)),
        dual.WanDHTOption(dht.RoutingTableRefreshPeriod(10*time.Minute)),
    )
    if err != nil {
        return nil, err
    }
    return d, d.Bootstrap(ctx)
}
```

**Рекомендация для Aether:**
- **Personal Node** → `ModeServer` + статический порт `/udp/4001/quic-v1`
- **Клиентские устройства** → `ModeAutoServer` (libp2p сам определит через AutoNAT)

### 2.2 AutoNAT — определение публичной доступности

```go
import autonat "github.com/libp2p/go-libp2p/p2p/host/autonat"

func SetupAutoNAT(h host.Host) (autonat.AutoNAT, error) {
    return autonat.New(h,
        autonat.EnableService(h.Network()),
        autonat.WithReachability(func(r network.Reachability) {
            switch r {
            case network.ReachabilityPublic:
                // → переключить DHT в Server mode
            case network.ReachabilityPrivate:
                // → включить Circuit Relay клиент
            case network.ReachabilityUnknown:
                // → повторить probe через 5 мин
            }
        }),
    )
}
```

**Логика переключения:**

```
AutoNAT probe (30-60 сек)
 ├─ Public  → DHT Server mode, relay отключён
 ├─ Private → DHT Client mode + Circuit Relay клиент + DCUTR
 └─ Unknown → DHT Client mode (консервативно), повтор через 5 мин
```

### 2.3 mDNS — обнаружение в локальной сети

```go
import "github.com/libp2p/go-libp2p/p2p/discovery/mdns"

type mdnsNotifee struct{ h host.Host }

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
    n.h.Connect(context.Background(), pi)
}

func StartMDNS(h host.Host) error {
    svc := mdns.NewMdnsService(h, "aether-messenger/1.0.0", &mdnsNotifee{h: h})
    return svc.Start()
}
```

Service tag `"aether-messenger/1.0.0"` изолирует обнаружение от других libp2p-приложений.

---

## 3. PEX (Peer Exchange)

### 3.1 Зачем PEX при наличии DHT

DHT — глобальный поиск, но дорого: каждый `FindPeer` = десятки UDP-пакетов. Для **доверенных контактов** PEX даёт:

- Латентность обнаружения: ~100-500 мс вместо 2-5 сек через DHT
- Независимость от DHT при локальных / изолированных сетях
- Снижение нагрузки на DHT-инфраструктуру

### 3.2 Протокол /aether/pex/1.0.0

```go
package pex

import (
    "bufio"
    "context"
    "encoding/json"
    "time"

    "github.com/libp2p/go-libp2p/core/host"
    "github.com/libp2p/go-libp2p/core/network"
    "github.com/libp2p/go-libp2p/core/peer"
    "github.com/libp2p/go-libp2p/core/protocol"
    ma "github.com/multiformats/go-multiaddr"
)

const PEXProtocolID = protocol.ID("/aether/pex/1.0.0")

type PeerRecord struct {
    ID    peer.ID  `json:"id"`
    Addrs []string `json:"addrs"` // multiaddress строки
}

type PEXMessage struct {
    Type  string       `json:"type"` // "request" | "response"
    Peers []PeerRecord `json:"peers,omitempty"`
}

type PEXService struct {
    host      host.Host
    trusted   *TrustedPeerStore
}

func NewPEXService(h host.Host, store *TrustedPeerStore) *PEXService {
    svc := &PEXService{host: h, trusted: store}
    h.SetStreamHandler(PEXProtocolID, svc.handleStream)
    return svc
}

func (s *PEXService) handleStream(stream network.Stream) {
    defer stream.Close()
    stream.SetDeadline(time.Now().Add(10 * time.Second))

    remotePeer := stream.Conn().RemotePeer()
    if !s.trusted.IsTrusted(remotePeer) {
        stream.Reset() // не доверяем — закрываем
        return
    }

    var req PEXMessage
    if err := json.NewDecoder(bufio.NewReader(stream)).Decode(&req); err != nil {
        return
    }

    peers := s.trusted.GetTrustedPeers(20)
    resp := PEXMessage{Type: "response", Peers: s.marshalPeers(peers)}
    json.NewEncoder(stream).Encode(resp)
}

func (s *PEXService) RequestPeers(ctx context.Context, target peer.ID) ([]peer.AddrInfo, error) {
    stream, err := s.host.NewStream(ctx, target, PEXProtocolID)
    if err != nil {
        return nil, err
    }
    defer stream.Close()
    stream.SetDeadline(time.Now().Add(10 * time.Second))

    json.NewEncoder(stream).Encode(PEXMessage{Type: "request"})

    var resp PEXMessage
    if err := json.NewDecoder(bufio.NewReader(stream)).Decode(&resp); err != nil {
        return nil, err
    }
    return s.unmarshalPeers(resp.Peers), nil
}

// RunPEXLoop — периодически опрашивает доверенных пиров.
func (s *PEXService) RunPEXLoop(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            for _, p := range s.trusted.GetOnlineTrustedPeers(5) {
                go func(target peer.ID) {
                    pctx, cancel := context.WithTimeout(ctx, 15*time.Second)
                    defer cancel()
                    newPeers, err := s.RequestPeers(pctx, target)
                    if err != nil {
                        return
                    }
                    for _, pi := range newPeers {
                        s.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.TempAddrTTL)
                    }
                }(p)
            }
        }
    }
}
```

### 3.3 Риски PEX

| Риск | Митигация |
|---|---|
| Скомпрометированный пир раскрывает адреса | Подписывать PeerRecord Ed25519-ключом владельца |
| PEX-флуд (тысячи записей) | Лимит 20 записей + rate limiting на handler |
| Устаревшие адреса | Использовать `peerstore.TempAddrTTL` (10 мин), не `Permanent` |

---

## 4. Relay: Circuit v2

### 4.1 Когда нужен Relay

Circuit Relay v2 — туннелирование трафика через публичный узел:

```
Alice (Symmetric NAT) ──► Relay ──► Bob (Symmetric NAT)
```

Используется когда DCUTR hole punching не удался.

### 4.2 Конфигурация Relay-клиента и сервера

```go
import (
    multiaddr "github.com/multiformats/go-multiaddr"
    "github.com/libp2p/go-libp2p/core/peer"
    relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
    "github.com/libp2p/go-libp2p/p2p/host/autorelay"
)

func getBootstrapRelays() []peer.AddrInfo {
    addrs := []string{
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
        "/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
    }
    var peers []peer.AddrInfo
    for _, s := range addrs {
        maddr, _ := multiaddr.NewMultiaddr(s)
        pi, _ := peer.AddrInfoFromP2pAddr(maddr)
        if pi != nil {
            peers = append(peers, *pi)
        }
    }
    return peers
}

// EnableRelayService — только для Personal Node с публичным IP!
func EnableRelayService(h host.Host) error {
    _, err := relayv2.New(h,
        relayv2.WithLimit(&relayv2.RelayLimit{
            Data:     512 * 1024,      // 512 KB per tunnel
            Duration: 2 * time.Hour,   // max reservation TTL
        }),
    )
    return err
}
```

### 4.3 Схема взаимодействия при Symmetric NAT

```
1. Alice: AutoNAT → "Private" → включает relay-клиент
2. Alice резервирует слот на Relay (RESERVE request)
3. Relay выдаёт relay-multiaddr с TTL 1 час
4. Alice анонсирует в DHT:
   /ip4/relay.host/tcp/443/p2p/RelayID/p2p-circuit/p2p/AliceID
5. Bob находит адрес Alice через DHT → подключается через relay
6. DCUTR пробует прямое соединение параллельно:
   - Успех → переключается на прямое
   - Неудача → relay остаётся основным каналом
```

### 4.4 Производительность Relay

| Параметр | Значение |
|---|---|
| Throughput (relay) | ~50-100 Mbps (зависит от bandwidth relay-ноды) |
| Latency overhead | +20-80 мс (зависит от гео) |
| Оптимально для сообщений | ≤ 64 KB (для мессенджера достаточно) |
| Relay reservation TTL | 1 час (auto-renewal) |

**Для Aether:** Personal Node пользователя должен выступать relay для его устройств — снижает latency и зависимость от публичных relay.

---

## 5. Архитектурная сводка и риски

### 5.1 Рекомендуемый стек для Aether

```
┌─────────────────────────────────────────────────────────┐
│                     Aether Node                         │
├─────────────────────────────────────────────────────────┤
│  Application:  /aether/msg/1.0  /aether/pex/1.0.0       │
├─────────────────────────────────────────────────────────┤
│  Discovery:    Kademlia DHT (WAN + LAN dual mode)        │
│                mDNS (local) · AutoNAT · DCUTR · PEX     │
├─────────────────────────────────────────────────────────┤
│  Security:     Noise protocol (Ed25519 identity)         │
├─────────────────────────────────────────────────────────┤
│  Multiplexing: yamux (over TCP)                          │
├─────────────────────────────────────────────────────────┤
│  Transport:    QUIC v1 (primary) / TCP (fallback)        │
│                WebSocket port 443 (corporate)            │
│                Circuit Relay v2 (Symmetric NAT)          │
└─────────────────────────────────────────────────────────┘
```

### 5.2 Таблица рисков

| Компонент | Риск | Вероятность | Митигация |
|---|---|---|---|
| QUIC | UDP заблокирован ISP | Средняя | TCP + WebSocket fallback |
| DHT | Sybil-атака | Низкая | Trust-based peer selection |
| DCUTR | Symmetric NAT (~30%) | Высокая | Circuit Relay v2 |
| PEX | Отравление peer list | Низкая | Ed25519 подписи записей |
| AutoNAT | False positive при CGN | Средняя | Крест-проверка ≥3 пирами |
| Relay | DoS relay-ноды | Средняя | Лимиты + Personal Node как relay |

### 5.3 Приоритеты реализации

| Этап | Задача | Сложность |
|---|---|---|
| 1 | Базовый хост (QUIC + TCP + Noise + yamux) | Низкая |
| 1 | DCUTR hole punching | Минимальная (одна опция libp2p) |
| 2 | DHT + AutoNAT | Средняя |
| 2 | mDNS LAN discovery | Низкая |
| 3 | Circuit Relay клиент | Низкая |
| 4 | PEX протокол (/aether/pex/1.0.0) | Средняя |
| 5 | Personal Node relay сервис | Средняя |

---

*Документ подготовлен для команды Aether. go-libp2p API может меняться — проверяйте CHANGELOG.*
