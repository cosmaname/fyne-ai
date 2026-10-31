package main

import (
	"image/color"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

const gridSize = 10

type cellEntry struct {
	widget.Entry
	row, col int
	grid     [][]*cellEntry
	window   fyne.Window
	editing  bool
	numeric  bool
}

func newCellEntry(row, col int, grid [][]*cellEntry, window fyne.Window) *cellEntry {
	c := &cellEntry{row: row, col: col, grid: grid, window: window}
	c.ExtendBaseWidget(c)
	c.Wrapping = fyne.TextWrapOff
	c.numeric = row == 0 || col == 0
	return c
}

func (c *cellEntry) TypedRune(r rune) {
	if c.numeric && (r < '0' || r > '9') {
		return
	}
	if !c.editing {
		c.editing = true
	}
	c.Entry.TypedRune(r)
}

func (c *cellEntry) TypedKey(ev *fyne.KeyEvent) {
	if c.editing {
		switch ev.Name {
		case fyne.KeyReturn, fyne.KeyEnter:
			c.editing = false
			return
		default:
			c.Entry.TypedKey(ev)
			return
		}
	}

	switch ev.Name {
	case fyne.KeyUp:
		c.moveFocus(-1, 0)
	case fyne.KeyDown:
		c.moveFocus(1, 0)
	case fyne.KeyLeft:
		c.moveFocus(0, -1)
	case fyne.KeyRight:
		c.moveFocus(0, 1)
	case fyne.KeyReturn, fyne.KeyEnter:
		c.editing = true
	default:
		c.Entry.TypedKey(ev)
	}
}

func (c *cellEntry) moveFocus(dr, dc int) {
	nr := c.row + dr
	nc := c.col + dc
	if nr < 0 || nr >= gridSize || nc < 0 || nc >= gridSize {
		return
	}

	next := c.grid[nr][nc]
	if next == nil {
		return
	}

	next.editing = false
	c.window.Canvas().Focus(next)
}

func (c *cellEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if _, ok := shortcut.(*desktop.CustomShortcut); ok {
		return
	}
	c.Entry.TypedShortcut(shortcut)
}

func (c *cellEntry) FocusLost() {
	c.Entry.FocusLost()
	if !c.numeric {
		return
	}

	t := strings.TrimSpace(c.Text)
	if t == "" {
		return
	}

	v, err := strconv.Atoi(t)
	if err != nil {
		c.SetText("")
		return
	}

	c.SetText(strconv.Itoa(v))
}

func main() {
	a := app.New()
	w := a.NewWindow("Hello")

	entries := make([][]*cellEntry, gridSize)
	for r := 0; r < gridSize; r++ {
		entries[r] = make([]*cellEntry, gridSize)
	}

	grid := container.NewGridWithColumns(gridSize)
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			entry := newCellEntry(r, c, entries, w)
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

	w.SetContent(grid)
	w.Resize(fyne.NewSize(900, 420))
	w.ShowAndRun()
}
