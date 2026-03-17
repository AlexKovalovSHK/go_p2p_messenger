package event

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_PubSub(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := bus.Subscribe(ctx, EventMessageReceived)

	go func() {
		bus.Publish(Event{
			Type: EventMessageReceived,
			Data: "hello world",
		})
	}()

	select {
	case ev := <-sub:
		assert.Equal(t, EventMessageReceived, ev.Type)
		assert.Equal(t, "hello world", ev.Data)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())

	sub := bus.Subscribe(ctx, EventSyncCompleted)
	
	cancel() // Unsubscribe

	// Wait for unsubscription to process
	time.Sleep(50 * time.Millisecond)

	bus.Publish(Event{Type: EventSyncCompleted, Data: 123})

	_, ok := <-sub
	assert.False(t, ok, "Channel should be closed after unsubscription")
}

func TestEventBus_MultipleTypes(t *testing.T) {
	bus := NewBus()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sub := bus.Subscribe(ctx, EventPeerConnected, EventPeerDisconnected)

	bus.Publish(Event{Type: EventPeerConnected, Data: "peer1"})
	ev1 := <-sub
	assert.Equal(t, EventPeerConnected, ev1.Type)

	bus.Publish(Event{Type: EventPeerDisconnected, Data: "peer1"})
	ev2 := <-sub
	assert.Equal(t, EventPeerDisconnected, ev2.Type)
}
