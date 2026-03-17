package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// AetherTheme provides a custom look for the messenger.
type AetherTheme struct{}

func (m AetherTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		if variant == theme.VariantDark {
			return color.NRGBA{R: 20, G: 20, B: 30, A: 255} // Dark Navy
		}
		return color.NRGBA{R: 245, G: 245, B: 250, A: 255}
	}
	if name == theme.ColorNamePrimary {
		return color.NRGBA{R: 110, G: 80, B: 255, A: 255} // Aether Purple
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (m AetherTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (m AetherTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (m AetherTheme) Size(name fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(name)
}
