package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// AppNavigator manages the application's window and primary navigation stack.
type AppNavigator struct {
	app    fyne.App
	window fyne.Window

	master *fyne.Container
	detail *fyne.Container
	split  *container.Split
}

// NewAppNavigator initializes the Fyne application and main window.
func NewAppNavigator(title string) *AppNavigator {
	a := app.New()
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(900, 600)) // Desktop-friendly landscape ratio

	// Master side (Chat List) - stays permanent
	master := container.NewStack()
	
	// Detail side (Direct Chat) - changes content
	detail := container.NewStack()

	split := container.NewHSplit(master, detail)
	split.Offset = 0.35 // 35% for the list

	nav := &AppNavigator{
		app:    a,
		window: w,
		master: master,
		detail: detail,
		split:  split,
	}

	nav.SetDefaultDetail()
	w.SetContent(split)
	return nav
}

// SetDefaultDetail resets the right pane to the initial welcome message.
func (n *AppNavigator) SetDefaultDetail() {
	n.SetDetail(container.NewCenter(
		widget.NewLabel("Выберите чат из списка слева или добавьте новый контакт"),
	))
}

// SetMaster sets the left pane (Chat List)
func (n *AppNavigator) SetMaster(content fyne.CanvasObject) {
	n.master.Objects = []fyne.CanvasObject{content}
	n.master.Refresh()
	n.split.Refresh()
}

// SetDetail sets the right pane (Chat View)
func (n *AppNavigator) SetDetail(content fyne.CanvasObject) {
	if content == nil {
		n.SetDefaultDetail()
		return
	}
	n.detail.Objects = []fyne.CanvasObject{content}
	n.detail.Refresh()
	n.split.Refresh()
}

// SetContent allows setting a full-screen view, replacing the split view
func (n *AppNavigator) SetContent(content fyne.CanvasObject) {
	n.window.SetContent(content)
}

// SetSplit restored the split view as the main window content
func (n *AppNavigator) SetSplit() {
	n.window.SetContent(n.split)
}

// ShowAndRun displays the window and starts the event loop.
func (n *AppNavigator) ShowAndRun() {
	n.window.ShowAndRun()
}
