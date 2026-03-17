package event

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBus_PubSub(t *testing.T) {
	bus := NewBus()

	sub := bus.Subscribe(TopicNewMessage)

	expectedData := MessageEvent{Text: "hello world"}

	go func() {
		// Small sleep to ensure subscriber is ready (though our implementation handles it)
		time.Sleep(10 * time.Millisecond)
		bus.Publish(TopicNewMessage, expectedData)
	}()

	select {
	case ev := <-sub:
		assert.Equal(t, expectedData, ev)
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for event")
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := NewBus()

	sub1 := bus.Subscribe(TopicSyncStatus)
	sub2 := bus.Subscribe(TopicSyncStatus)

	expected := SyncEvent{Status: "completed"}

	go func() {
		time.Sleep(10 * time.Millisecond)
		bus.Publish(TopicSyncStatus, expected)
	}()

	select {
	case ev1 := <-sub1:
		assert.Equal(t, expected, ev1)
	case <-time.After(time.Second):
		t.Fatal("Sub1 timeout")
	}

	select {
	case ev2 := <-sub2:
		assert.Equal(t, expected, ev2)
	case <-time.After(time.Second):
		t.Fatal("Sub2 timeout")
	}
}
