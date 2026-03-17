package event

import (
	"sync"
)

const (
	// Chat Events
	TopicNewMessage = "chat.message.new"
	TopicNewContact = "chat.contact.new"
	
	// Sync Events
	TopicSyncStatus = "sync.status"
	
	// Connection Events
	TopicNodeReachability = "node.reachability"
)

// MessageEvent payload for new messages
type MessageEvent struct {
	ID         string
	ChatID     string
	SenderID   string
	Text       string
	Timestamp  int64
	IsIncoming bool
	Status     string
}

// SyncEvent payload for synchronization progress
type SyncEvent struct {
	Status  string // "started", "completed", "failed"
	Progress float64
}

// Bus представляет потокобезопасную шину событий.
type Bus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan interface{}
}

// NewBus создает новый экземпляр шины событий.
func NewBus() *Bus {
	return &Bus{
		subscribers: make(map[string][]chan interface{}),
	}
}

// Publish отправляет данные во все каналы, подписанные на указанный топик.
func (b *Bus) Publish(topic string, data interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if chans, found := b.subscribers[topic]; found {
		for _, ch := range chans {
			// Отправка в неблокирующем режиме, чтобы медленные подписчики не тормозили систему.
			select {
			case ch <- data:
			default:
				// Буфер полон, сообщение пропускается.
			}
		}
	}
}

// Subscribe создает канал и подписывает его на указанный топик.
func (b *Bus) Subscribe(topic string) chan interface{} {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan interface{}, 100) // Буферизация для стабильности
	b.subscribers[topic] = append(b.subscribers[topic], ch)
	return ch
}
