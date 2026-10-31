package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func main() {
	a := app.New()
	w := a.NewWindow("Hello")

	btn := widget.NewButton("Click me", nil)
	btn.OnTapped = func() {
		btn.SetText("Hello, world!")
	}

	w.SetContent(container.NewCenter(btn))
	w.Resize(fyne.NewSize(320, 180))
	w.ShowAndRun()
}
