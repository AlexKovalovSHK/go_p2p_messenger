package screens

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/user/aether/internal/api"
	"github.com/user/aether/internal/ui/viewmodel"
)

// DirectChatScreen displays messages and an input field.
type DirectChatScreen struct {
	vm      *viewmodel.DirectChatViewModel
	chatSvc *api.ChatService
	onBack  func()
}

func NewDirectChatScreen(vm *viewmodel.DirectChatViewModel, chatSvc *api.ChatService, onBack func()) *DirectChatScreen {
	return &DirectChatScreen{vm: vm, chatSvc: chatSvc, onBack: onBack}
}

func (s *DirectChatScreen) Render() fyne.CanvasObject {
	list := widget.NewListWithData(
		s.vm.Messages,
		func() fyne.CanvasObject {
			return widget.NewLabel("Message body")
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			val, _ := i.(binding.Untyped).Get()
			msg := val.(api.MessageDTO)
			
			label := o.(*widget.Label)
			label.SetText(msg.Content)
			if msg.IsOwn {
				label.Alignment = fyne.TextAlignTrailing
			} else {
				label.Alignment = fyne.TextAlignLeading
			}
		},
	)

	input := widget.NewEntry()
	input.SetPlaceHolder("Type a message...")
	
	sendBtn := widget.NewButton("Send", func() {
		if input.Text == "" {
			return
		}
		pID, _ := peer.Decode(s.vm.GetPeerID())
		_ = pID 
		
		// Note: In real app we need recipientPubKey too. 
		// For MVP, we'll assume we know it or have a getter in api.
		// s.chatSvc.SendMessage(context.Background(), pID, nil, input.Text)
		input.SetText("")
	})

	footer := container.NewBorder(nil, nil, nil, sendBtn, input)
	
	header := container.NewHBox(
		widget.NewIcon(theme.AccountIcon()),
		widget.NewLabelWithStyle(s.vm.GetPeerID(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		widget.NewLabel("● Online"),
	)

	return container.NewBorder(header, footer, nil, nil, list)
}
