package screens

import (
	"context"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/user/aether/internal/api"
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

	title := widget.NewLabelWithStyle("Aether Messenger", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})
	
	idLabel := widget.NewLabel("Your Device ID:")
	idValue := widget.NewEntry()
	idValue.SetText(status.DeviceID)
	idValue.Disable()

	copyBtn := widget.NewButtonWithIcon("Copy ID", theme.ContentCopyIcon(), func() {
		fyne.CurrentApp().Driver().AllWindows()[0].Clipboard().SetContent(status.DeviceID)
	})

	genBtn := widget.NewButton("Generate New Identity", func() {
		s.nodeSvc.GenerateNewIdentity()
		idValue.SetText(s.nodeSvc.GetStatus(context.Background()).DeviceID)
	})

	exportBtn := widget.NewButton("Export Identity", func() {
		// In a real app we would use dialog.ShowFileSave
		bytes, _ := s.nodeSvc.ExportIdentity()
		_ = bytes
		log.Println("Identity exported to log (simulated)")
	})

	startBtn := widget.NewButtonWithIcon("Enter Messenger", theme.ConfirmIcon(), s.onStart)
	startBtn.Importance = widget.HighImportance

	content := container.NewVBox(
		container.NewCenter(title),
		widget.NewSeparator(),
		idLabel,
		container.NewBorder(nil, nil, nil, copyBtn, idValue),
		container.NewHBox(genBtn, exportBtn),
		layout.NewSpacer(),
		startBtn,
	)

	return container.NewPadded(content)
}
