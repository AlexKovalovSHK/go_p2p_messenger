package viewmodel

import (
	"fyne.io/fyne/v2/data/binding"
)

// MainViewModel manages global application state like current navigation.
type MainViewModel struct {
	CurrentChatID binding.String
}

func NewMainViewModel() *MainViewModel {
	return &MainViewModel{
		CurrentChatID: binding.NewString(),
	}
}
