package main

import (
	"database/sql"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
)

const gridSize = 10

var columnNames = [...]string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"}

type gridStore struct {
	db *sql.DB
}

func newGridStore(path string) (*gridStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	store := &gridStore{db: db}
	if err := store.init(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *gridStore) Close() error {
	return s.db.Close()
}

func (s *gridStore) init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS grid_rows (
			id INTEGER PRIMARY KEY,
			one TEXT NOT NULL DEFAULT '',
			two TEXT NOT NULL DEFAULT '',
			three TEXT NOT NULL DEFAULT '',
			four TEXT NOT NULL DEFAULT '',
			five TEXT NOT NULL DEFAULT '',
			six TEXT NOT NULL DEFAULT '',
			seven TEXT NOT NULL DEFAULT '',
			eight TEXT NOT NULL DEFAULT '',
			nine TEXT NOT NULL DEFAULT '',
			ten TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return err
	}

	for i := 1; i <= gridSize; i++ {
		if _, err := s.db.Exec(`INSERT INTO grid_rows (id) VALUES (?) ON CONFLICT(id) DO NOTHING`, i); err != nil {
			return err
		}
	}

	return nil
}

func (s *gridStore) load(entries [][]*cellEntry) error {
	rows, err := s.db.Query(`SELECT id, one, two, three, four, five, six, seven, eight, nine, ten FROM grid_rows ORDER BY id ASC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var values [gridSize]string
		if err := rows.Scan(
			&id,
			&values[0],
			&values[1],
			&values[2],
			&values[3],
			&values[4],
			&values[5],
			&values[6],
			&values[7],
			&values[8],
			&values[9],
		); err != nil {
			return err
		}

		if id < 1 || id > gridSize {
			continue
		}

		for col := 0; col < gridSize; col++ {
			entries[id-1][col].SetText(values[col])
		}
	}

	return rows.Err()
}

func (s *gridStore) saveCell(row, col int, value string) error {
	if row < 0 || row >= gridSize || col < 0 || col >= gridSize {
		return nil
	}

	query := fmt.Sprintf("UPDATE grid_rows SET %s = ? WHERE id = ?", columnNames[col])
	_, err := s.db.Exec(query, value, row+1)
	return err
}

type cellEntry struct {
	widget.Entry
	row, col int
	grid     [][]*cellEntry
	window   fyne.Window
	store    *gridStore
	editing  bool
	numeric  bool
}

func newCellEntry(row, col int, grid [][]*cellEntry, window fyne.Window, store *gridStore) *cellEntry {
	c := &cellEntry{row: row, col: col, grid: grid, window: window, store: store}
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
			c.commit()
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
	c.commit()
}

func (c *cellEntry) commit() {
	if c.numeric {
		t := strings.TrimSpace(c.Text)
		if t == "" {
			c.SetText("")
		} else {
			v, err := strconv.Atoi(t)
			if err != nil {
				c.SetText("")
			} else {
				c.SetText(strconv.Itoa(v))
			}
		}
	}

	if c.store != nil {
		if err := c.store.saveCell(c.row, c.col, c.Text); err != nil {
			log.Printf("failed saving cell (%d,%d): %v", c.row, c.col, err)
		}
	}
}

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

	w.SetContent(grid)
	w.Resize(fyne.NewSize(900, 420))
	w.ShowAndRun()
}
