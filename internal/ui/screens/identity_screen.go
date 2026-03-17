package screens

import (
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/user/aether/internal/api"
	"image/color"
)

// IdentityScreen displays the user's Device ID and connection info.
type IdentityScreen struct {
	nodeSvc *api.NodeService
	onStart func()
}

func NewIdentityScreen(ns *api.NodeService, onStart func()) *IdentityScreen {
	return &IdentityScreen{nodeSvc: ns, onStart: onStart}
}

func (s *IdentityScreen) Render() fyne.CanvasObject {
	status := s.nodeSvc.GetStatus(context.Background())

	title := canvas.NewText("Aether", color.White)
	title.TextSize = 32
	title.Alignment = fyne.TextAlignCenter

	idLabel := widget.NewLabelWithStyle("Device ID", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	idValue := widget.NewEntry()
	idValue.SetText(status.DeviceID)
	idValue.Disable()

	copyBtn := widget.NewButton("Copy ID", func() {
		fyne.CurrentApp().Driver().AllWindows()[0].Clipboard().SetContent(status.DeviceID)
	})

	startBtn := widget.NewButtonWithIcon("Enter Chat", nil, s.onStart)
	startBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		idLabel,
		idValue,
		copyBtn,
		layout.NewSpacer(),
		startBtn,
	)

	return container.NewPadded(content)
}
