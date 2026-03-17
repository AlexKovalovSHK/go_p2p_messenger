package viewmodel

import (
	"context"
	"log"

	"fyne.io/fyne/v2/data/binding"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/event"
)

// ChatListViewModel handles data binding for the conversation list.
type ChatListViewModel struct {
	chatSvc *api.ChatService
	bus     event.Bus

	Conversations binding.UntypedList
}

func NewChatListViewModel(chatSvc *api.ChatService, bus event.Bus) *ChatListViewModel {
	return &ChatListViewModel{
		chatSvc:       nil, // Placeholder for actual injection
		bus:           bus,
		Conversations: binding.NewUntypedList(),
	}
}

// Refresh reloads the conversation list from the API.
func (vm *ChatListViewModel) Refresh(ctx context.Context, chatSvc *api.ChatService) {
	convs, err := chatSvc.ListConversations(ctx)
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

// Watch listens for new messages to trigger refreshes.
func (vm *ChatListViewModel) Watch(ctx context.Context, chatSvc *api.ChatService) {
	ch := vm.bus.Subscribe(ctx, event.EventMessageReceived, event.EventMessageSent)
	go func() {
		for range ch {
			vm.Refresh(ctx, chatSvc)
		}
	}()
}
