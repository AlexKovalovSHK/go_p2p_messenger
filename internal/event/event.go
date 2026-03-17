package event

import (
	"context"
	"sync"
)

// EventType distinguishes between different types of events in the system.
type EventType string

const (
	EventMessageReceived    EventType = "message_received"
	EventMessageSent        EventType = "message_sent"
	EventNodeReachability   EventType = "node_reachability"
	EventPeerConnected      EventType = "peer_connected"
	EventPeerDisconnected   EventType = "peer_disconnected"
	EventSyncStarted        EventType = "sync_started"
	EventSyncCompleted      EventType = "sync_completed"
	EventSyncFailed         EventType = "sync_failed"
)

// Event carries a payload for a specific type.
type Event struct {
	Type EventType
	Data interface{}
}

// Bus defines the interface for an internal event-driven communication.
type Bus interface {
	Publish(event Event)
	Subscribe(ctx context.Context, types ...EventType) <-chan Event
}

// channelBus is a simple implementation of Bus using Go channels.
type channelBus struct {
	mu          sync.RWMutex
	subscribers map[EventType][]chan Event
}

// NewBus creates a new instance of the event bus.
func NewBus() Bus {
	return &channelBus{
		subscribers: make(map[EventType][]chan Event),
	}
}

// Publish sends an event to all interested subscribers.
// It is non-blocking for the publisher.
func (b *channelBus) Publish(event Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	subs := b.subscribers[event.Type]
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Buffer full, skip to avoid blocking the system
		}
	}
}

// Subscribe returns a channel that receive events of the specified types.
// The subscription is automatically cleaned up when the context is cancelled.
func (b *channelBus) Subscribe(ctx context.Context, types ...EventType) <-chan Event {
	ch := make(chan Event, 32) // Buffered channel for robustness

	b.mu.Lock()
	for _, t := range types {
		b.subscribers[t] = append(b.subscribers[t], ch)
	}
	b.mu.Unlock()

	// Handle unsubscription
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		defer b.mu.Unlock()

		for _, t := range types {
			subs := b.subscribers[t]
			for i, s := range subs {
				if s == ch {
					// Remove from slice
					b.subscribers[t] = append(subs[:i], subs[i+1:]...)
					break
				}
			}
		}
		close(ch)
	}()

	return ch
}
