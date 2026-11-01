package main

import (
	"image/color"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
)

func main() {
	a := app.New()
	w := a.NewWindow("Hello")

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	store, err := newGridStore(filepath.Join(wd, "grid.sqlite"))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("failed to close sqlite db: %v", err)
		}
	}()

	entries := make([][]*cellEntry, gridSize)
	for r := 0; r < gridSize; r++ {
		entries[r] = make([]*cellEntry, gridSize)
	}

	grid := container.NewGridWithColumns(gridSize)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			entry := newCellEntry(r, c, entries, w, store)
			entry.Resize(fyne.NewSize(80, 32))
			entries[r][c] = entry

			if r == 0 || c == 0 {
				bg := canvas.NewRectangle(color.NRGBA{R: 232, G: 236, B: 243, A: 255})
				grid.Add(container.New(layout.NewStackLayout(), bg, entry))
			} else {
				grid.Add(entry)
			}
		}
	}

	if err := store.load(entries); err != nil {
		log.Fatal(err)
	}
	applyReferenceHeaders(entries)
	recalculateGrid(entries)

	w.SetContent(grid)
	w.Resize(fyne.NewSize(900, 420))
	w.ShowAndRun()
}
