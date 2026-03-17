package screens

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/ui/viewmodel"
)

// ChatListScreen displays a list of active conversations.
type ChatListScreen struct {
	vm      *viewmodel.ChatListViewModel
	onSelect func(string)
}

func NewChatListScreen(vm *viewmodel.ChatListViewModel, onSelect func(string)) *ChatListScreen {
	return &ChatListScreen{vm: vm, onSelect: onSelect}
}

func (s *ChatListScreen) Render() fyne.CanvasObject {
	list := widget.NewListWithData(
		s.vm.Conversations,
		func() fyne.CanvasObject {
			return container.NewVBox(
				widget.NewLabelWithStyle("PeerID", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
				widget.NewLabel("Last message..."),
			)
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			val, _ := i.(binding.Untyped).Get()
			conv := val.(api.ConversationDTO)
			
			labels := o.(*fyne.Container).Objects
			labels[0].(*widget.Label).SetText(conv.ID)
			labels[1].(*widget.Label).SetText(conv.LastMessage)
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		val, _ := s.vm.Conversations.GetValue(id)
		conv := val.(api.ConversationDTO)
		s.onSelect(conv.ID)
	}

	header := widget.NewLabelWithStyle("Chats", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	return container.NewBorder(header, nil, nil, nil, list)
}
