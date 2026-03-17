package viewmodel

import (
	"context"
	"log"

	"fyne.io/fyne/v2/data/binding"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/event"
)

// DirectChatViewModel manages the message list for a specific peer.
type DirectChatViewModel struct {
	chatSvc *api.ChatService
	bus     *event.Bus
	peerID  string

	Messages binding.UntypedList
}

func NewDirectChatViewModel(chatSvc *api.ChatService, bus *event.Bus, peerID string) *DirectChatViewModel {
	return &DirectChatViewModel{
		chatSvc:  chatSvc,
		bus:      bus,
		peerID:   peerID,
		Messages: binding.NewUntypedList(),
	}
}

func (vm *DirectChatViewModel) GetPeerID() string {
	return vm.peerID
}

// LoadMessages fetches history for the current conversation.
func (vm *DirectChatViewModel) LoadMessages(ctx context.Context) {
	msgs, err := vm.chatSvc.GetMessages(ctx, vm.peerID, 0, 50)
	if err != nil {
		log.Printf("VM: Failed to load messages: %v", err)
		return
	}

	data := make([]interface{}, len(msgs))
	for i, m := range msgs {
		data[i] = m
	}
	vm.Messages.Set(data)
}

// Watch listens for new messages in this conversation.
func (vm *DirectChatViewModel) Watch(ctx context.Context) {
	ch := vm.bus.Subscribe(event.TopicNewMessage)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-ch:
				// If event contains ChatID, we could filter here.
				// For now, reload always ensures we have latest.
				_ = ev
				vm.LoadMessages(ctx)
			}
		}
	}()
}
