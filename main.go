package main

import (
	"flag"
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
	cols := flag.Int("cols", 16, "columns including the header (2-27)")
	rows := flag.Int("rows", 16, "rows including the header (at least 2)")
	flag.Parse()
	if *cols < 2 || *cols > 27 {
		log.Fatalf("columns must be between 2 and 27 (header plus A-Z); got %d", *cols)
	}
	if *rows < 2 {
		log.Fatalf("rows must be at least 2; got %d", *rows)
	}
	size := gridSize{rows: *rows, cols: *cols}

	a := app.New()
	w := a.NewWindow("Hello")

	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	store, err := newGridStore(filepath.Join(wd, "grid.sqlite"), size)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Printf("failed to close sqlite db: %v", err)
		}
	}()

	entries := make([][]*cellEntry, size.rows)
	for r := range entries {
		entries[r] = make([]*cellEntry, size.cols)
	}

	grid := container.NewGridWithColumns(size.cols)
	for r := 0; r < size.rows; r++ {
		for c := 0; c < size.cols; c++ {
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
