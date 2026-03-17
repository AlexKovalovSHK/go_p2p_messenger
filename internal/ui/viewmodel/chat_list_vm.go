package viewmodel

import (
	"context"
	"log"

	"fyne.io/fyne/v2/data/binding"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/event"
)

// ChatListViewModel handles data binding for the conversation list and node status.
type ChatListViewModel struct {
	chatSvc *api.ChatService
	bus     *event.Bus

	Conversations binding.UntypedList
	NodeStatus    binding.String
}

func NewChatListViewModel(chatSvc *api.ChatService, bus *event.Bus) *ChatListViewModel {
	vm := &ChatListViewModel{
		chatSvc:       chatSvc,
		bus:           bus,
		Conversations: binding.NewUntypedList(),
		NodeStatus:    binding.NewString(),
	}
	vm.NodeStatus.Set("Offline")
	return vm
}

// Refresh reloads the conversation list from the API.
func (vm *ChatListViewModel) Refresh(ctx context.Context) {
	convs, err := vm.chatSvc.ListConversations(ctx)
	if err != nil {
		log.Printf("VM: Failed to list conversations: %v", err)
		return
	}

	data := make([]interface{}, len(convs))
	for i, c := range convs {
		data[i] = c
	}
	vm.Conversations.Set(data)
}

// AddContact proxy to ChatService
func (vm *ChatListViewModel) AddContact(ctx context.Context, peerID string) error {
	return vm.chatSvc.AddContact(ctx, peerID)
}

// Watch listens for new messages, new contacts and node reachability to trigger refreshes.
func (vm *ChatListViewModel) Watch(ctx context.Context) {
	msgCh := vm.bus.Subscribe(event.TopicNewMessage)
	contactCh := vm.bus.Subscribe(event.TopicNewContact)
	reachCh := vm.bus.Subscribe(event.TopicNodeReachability)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-msgCh:
				vm.Refresh(ctx)
			case <-contactCh:
				vm.Refresh(ctx)
			case ev := <-reachCh:
				if status, ok := ev.(string); ok {
					vm.NodeStatus.Set(status)
				}
			}
		}
	}()
}
