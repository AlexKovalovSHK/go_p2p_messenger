package transport

import (
	"context"
	"fmt"
	"sync"

	"github.com/libp2p/go-libp2p/core/peer"
)

// SentMessage is a record for MockTransport testing.
type SentMessage struct {
	To      peer.ID
	Payload []byte
}

// MockTransport allows mocking MessageTransport for unit tests.
type MockTransport struct {
	SentMessages []SentMessage
	handler      func(from peer.ID, payload []byte)
	mu           sync.Mutex
}

// NewMockTransport creates an isolated transport for tests.
func NewMockTransport() *MockTransport {
	return &MockTransport{
		SentMessages: make([]SentMessage, 0),
	}
}

// Send records the sent message natively.
func (m *MockTransport) Send(ctx context.Context, to peer.ID, payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = append(m.SentMessages, SentMessage{To: to, Payload: append([]byte(nil), payload...)})
	return nil
}

// Connect mock implementation.
func (m *MockTransport) Connect(ctx context.Context, to peer.ID) error {
	return nil
}

// Subscribe listens.
func (m *MockTransport) Subscribe(handler func(from peer.ID, payload []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

// Start mock
func (m *MockTransport) Start(ctx context.Context) error {
	return nil
}

// Stop mock
func (m *MockTransport) Stop() error {
	return nil
}

// SimulateIncoming enables pushing mock messages.
func (m *MockTransport) SimulateIncoming(from peer.ID, payload []byte) error {
	m.mu.Lock()
	h := m.handler
	m.mu.Unlock()
	if h != nil {
		h(from, payload)
		return nil
	}
	return fmt.Errorf("no subscriber handles incoming message")
}
