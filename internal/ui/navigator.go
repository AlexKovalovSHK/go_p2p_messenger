package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
)

// AppNavigator manages the application's window and primary navigation stack.
type AppNavigator struct {
	app    fyne.App
	window fyne.Window

	content *fyne.Container
}

// NewAppNavigator initializes the Fyne application and main window.
func NewAppNavigator(title string) *AppNavigator {
	a := app.New()
	w := a.NewWindow(title)
	w.Resize(fyne.NewSize(400, 700)) // Mobile-like portrait aspect ratio

	nav := &AppNavigator{
		app:    a,
		window: w,
		content: container.NewStack(),
	}

	w.SetContent(nav.content)
	return nav
}

// Push adds a new screen to the top of the stack.
func (n *AppNavigator) Push(screen fyne.CanvasObject) {
	n.content.Objects = []fyne.CanvasObject{screen}
	n.content.Refresh()
}

// ShowAndRun displays the window and starts the event loop.
func (n *AppNavigator) ShowAndRun() {
	n.window.ShowAndRun()
}
