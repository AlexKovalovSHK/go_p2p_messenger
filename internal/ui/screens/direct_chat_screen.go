package screens

import (
	"context"

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
			// A simple bubble: Card with Label
			return container.NewHBox(
				widget.NewCard("", "", widget.NewLabel("")),
			)
		},
		func(i binding.DataItem, o fyne.CanvasObject) {
			val, _ := i.(binding.Untyped).Get()
			msg := val.(api.MessageDTO)
			
			box := o.(*fyne.Container)
			card := box.Objects[0].(*widget.Card)
			card.SetContent(widget.NewLabel(msg.Content))
			
			if msg.IsOwn {
				box.Layout = layout.NewHBoxLayout()
				box.Objects = []fyne.CanvasObject{layout.NewSpacer(), card}
			} else {
				box.Layout = layout.NewHBoxLayout()
				box.Objects = []fyne.CanvasObject{card, layout.NewSpacer()}
			}
		},
	)

	input := widget.NewEntry()
	input.SetPlaceHolder("Type a message...")
	
	onSend := func() {
		if input.Text == "" {
			return
		}
		pID, err := peer.Decode(s.vm.GetPeerID())
		if err != nil {
			return
		}
		
		go func() {
			_, _ = s.chatSvc.SendMessage(context.Background(), pID, nil, input.Text)
		}()
		input.SetText("")
	}
	
	sendBtn := widget.NewButtonWithIcon("", theme.MailSendIcon(), onSend)
	input.OnSubmitted = func(_ string) { onSend() }

	footer := container.NewBorder(nil, nil, nil, sendBtn, input)
	
	header := container.NewHBox(
		widget.NewButtonWithIcon("", theme.NavigateBackIcon(), s.onBack),
		widget.NewIcon(theme.AccountIcon()),
		widget.NewLabelWithStyle(s.vm.GetPeerID(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		layout.NewSpacer(),
		widget.NewLabel("● Online"),
	)

	return container.NewBorder(header, footer, nil, nil, list)
}
