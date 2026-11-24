package main

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
	_ "github.com/mattn/go-sqlite3"
)

type gridSize struct {
	rows, cols int
}

type gridStore struct {
	db   *sql.DB
	size gridSize
}

func newGridStore(path string, size gridSize) (*gridStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	store := &gridStore{db: db, size: size}
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
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		CREATE TABLE IF NOT EXISTS grid_rows (
			id INTEGER PRIMARY KEY
		)
	`)
	if err != nil {
		return err
	}

	columns, err := tx.Query("PRAGMA table_info(grid_rows)")
	if err != nil {
		return err
	}
	existing := make(map[string]bool)
	for columns.Next() {
		var id, notNull, primaryKey int
		var name, dataType string
		var defaultValue sql.NullString
		if err := columns.Scan(&id, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			columns.Close()
			return err
		}
		existing[name] = true
	}
	err = columns.Err()
	columns.Close()
	if err != nil {
		return err
	}
	// Keep the old schema's data, including columns hidden by a smaller grid.
	for col, oldName := range []string{"one", "two", "three", "four", "five", "six", "seven", "eight", "nine", "ten"} {
		if !existing[oldName] {
			continue
		}
		name := columnName(col + 1)
		if existing[name] {
			return fmt.Errorf("cannot migrate %s: %s already exists", oldName, name)
		}
		if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE grid_rows RENAME COLUMN %s TO %s", oldName, name)); err != nil {
			return err
		}
		existing[name] = true
	}
	for col := 1; col < s.size.cols; col++ {
		name := columnName(col)
		if !existing[name] {
			if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE grid_rows ADD COLUMN %s TEXT NOT NULL DEFAULT ''", name)); err != nil {
				return err
			}
		}
	}

	for i := 1; i < s.size.rows; i++ {
		if _, err := tx.Exec(`INSERT INTO grid_rows (id) VALUES (?) ON CONFLICT(id) DO NOTHING`, i); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (s *gridStore) load(entries [][]*cellEntry) error {
	names := make([]string, s.size.cols-1)
	for col := 1; col < s.size.cols; col++ {
		names[col-1] = columnName(col)
	}
	query := fmt.Sprintf("SELECT id, %s FROM grid_rows WHERE id >= 1 AND id < ? ORDER BY id ASC", strings.Join(names, ", "))
	rows, err := s.db.Query(query, s.size.rows)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		values := make([]string, len(names))
		dest := make([]any, s.size.cols)
		dest[0] = &id
		for col := range values {
			dest[col+1] = &values[col]
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}

		if id < 1 || id >= s.size.rows {
			continue
		}

		for col := 1; col < s.size.cols; col++ {
			entries[id][col].raw = values[col-1]
		}
	}

	return rows.Err()
}

func (s *gridStore) saveCell(row, col int, value string) error {
	if row <= 0 || row >= s.size.rows || col <= 0 || col >= s.size.cols {
		return nil
	}

	query := fmt.Sprintf("UPDATE grid_rows SET %s = ? WHERE id = ?", columnName(col))
	_, err := s.db.Exec(query, value, row)
	return err
}

func columnName(col int) string {
	return fmt.Sprintf("col_%d", col)
}

type cellEntry struct {
	widget.Entry
	row, col int
	address  string
	grid     [][]*cellEntry
	window   fyne.Window
	store    *gridStore
	header   bool
	editing  bool
	raw      string
}

func newCellEntry(row, col int, grid [][]*cellEntry, window fyne.Window, store *gridStore) *cellEntry {
	c := &cellEntry{row: row, col: col, grid: grid, window: window, store: store}
	c.ExtendBaseWidget(c)
	c.Wrapping = fyne.TextWrapOff
	c.header = row == 0 || col == 0
	if row > 0 && col > 0 {
		c.address = fmt.Sprintf("%c%d", rune('A'+col-1), row)
	}
	return c
}

func (c *cellEntry) TypedRune(r rune) {
	if c.header {
		return
	}
	if !c.editing {
		c.editing = true
	}
	c.Entry.TypedRune(r)
}

func (c *cellEntry) TypedKey(ev *fyne.KeyEvent) {
	if c.header {
		switch ev.Name {
		case fyne.KeyUp:
			c.moveFocus(-1, 0)
		case fyne.KeyDown:
			c.moveFocus(1, 0)
		case fyne.KeyLeft:
			c.moveFocus(0, -1)
		case fyne.KeyRight:
			c.moveFocus(0, 1)
		}
		return
	}

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
		c.SetText(c.raw)
		c.editing = true
	default:
		c.Entry.TypedKey(ev)
	}
}

func (c *cellEntry) moveFocus(dr, dc int) {
	nr := c.row + dr
	nc := c.col + dc
	if nr < 0 || nr >= len(c.grid) || nc < 0 || nc >= len(c.grid[nr]) {
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
	if c.header {
		return
	}
	if _, ok := shortcut.(*desktop.CustomShortcut); ok {
		return
	}
	c.Entry.TypedShortcut(shortcut)
}

func (c *cellEntry) FocusLost() {
	c.Entry.FocusLost()
	if c.editing {
		c.editing = false
		c.commit()
	}
}

func (c *cellEntry) commit() {
	if c.header {
		return
	}
	c.raw = c.Text

	if c.store != nil {
		if err := c.store.saveCell(c.row, c.col, c.raw); err != nil {
			log.Printf("failed saving cell (%d,%d): %v", c.row, c.col, err)
		}
	}

	recalculateGrid(c.grid)
}

func applyReferenceHeaders(entries [][]*cellEntry) {
	entries[0][0].raw = ""
	entries[0][0].SetText("")
	for col := 1; col < len(entries[0]); col++ {
		header := string(rune('A' + col - 1))
		entries[0][col].raw = header
		entries[0][col].SetText(header)
	}
	for row := 1; row < len(entries); row++ {
		header := fmt.Sprintf("%d", row)
		entries[row][0].raw = header
		entries[row][0].SetText(header)
	}
}

func recalculateGrid(entries [][]*cellEntry) {
	memo := make(map[string]string)
	visiting := make(map[string]bool)

	for row := 1; row < len(entries); row++ {
		for col := 1; col < len(entries[row]); col++ {
			entries[row][col].SetText(evaluateCell(entries, row, col, memo, visiting))
		}
	}
}

func evaluateCell(entries [][]*cellEntry, row, col int, memo map[string]string, visiting map[string]bool) string {
	cell := entries[row][col]
	if val, ok := memo[cell.address]; ok {
		return val
	}
	if visiting[cell.address] {
		return "#CYCLE!"
	}

	visiting[cell.address] = true
	rawTrimmed := strings.TrimSpace(cell.raw)
	value := cell.raw

	if strings.HasPrefix(rawTrimmed, "=") {
		value = evaluateFormula(entries, rawTrimmed[1:], memo, visiting)
	}

	memo[cell.address] = value
	delete(visiting, cell.address)
	return value
}

func evaluateFormula(entries [][]*cellEntry, expr string, memo map[string]string, visiting map[string]bool) string {
	terms, ops, ok := parseFormulaTerms(expr)
	if !ok || len(terms) == 0 || len(terms) != len(ops) {
		return "#REF!"
	}

	hasMinus := false
	for _, op := range ops {
		if op == '-' {
			hasMinus = true
			break
		}
	}

	var (
		typeSet      bool
		numericType  bool
		numericTotal int
		textTotal    strings.Builder
	)

	for i, term := range terms {
		refRow, refCol, ok := parseCellRef(term, gridSize{rows: len(entries), cols: len(entries[0])})
		if !ok {
			return "#REF!"
		}

		value := evaluateCell(entries, refRow, refCol, memo, visiting)
		if strings.HasPrefix(value, "#") {
			return value
		}

		intVal, err := strconv.Atoi(strings.TrimSpace(value))
		isNumeric := err == nil

		if !typeSet {
			typeSet = true
			numericType = isNumeric
		} else if numericType != isNumeric {
			return "#REF!"
		}

		if isNumeric {
			if ops[i] == '-' {
				numericTotal -= intVal
			} else {
				numericTotal += intVal
			}
		} else {
			if hasMinus {
				return "#REF!"
			}
			textTotal.WriteString(value)
		}
	}

	if !typeSet {
		return "#REF!"
	}

	if numericType {
		return strconv.Itoa(numericTotal)
	}

	return textTotal.String()
}

func parseFormulaTerms(expr string) ([]string, []rune, bool) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return nil, nil, false
	}

	terms := make([]string, 0, 4)
	ops := make([]rune, 0, 4)

	i := 0
	currentOp := rune('+')
	expectTerm := true

	for i < len(trimmed) {
		for i < len(trimmed) && trimmed[i] == ' ' {
			i++
		}
		if i >= len(trimmed) {
			break
		}

		if expectTerm {
			if trimmed[i] == '+' || trimmed[i] == '-' {
				if len(terms) > 0 {
					return nil, nil, false
				}
				currentOp = rune(trimmed[i])
				i++
				continue
			}

			start := i
			for i < len(trimmed) && trimmed[i] != '+' && trimmed[i] != '-' {
				i++
			}

			term := strings.TrimSpace(trimmed[start:i])
			if term == "" {
				return nil, nil, false
			}

			terms = append(terms, term)
			ops = append(ops, currentOp)
			currentOp = '+'
			expectTerm = false
			continue
		}

		if trimmed[i] != '+' && trimmed[i] != '-' {
			return nil, nil, false
		}
		currentOp = rune(trimmed[i])
		i++
		expectTerm = true
	}

	if expectTerm {
		return nil, nil, false
	}

	return terms, ops, true
}

func parseCellRef(ref string, size gridSize) (int, int, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(ref))
	if len(normalized) < 2 {
		return 0, 0, false
	}

	colRune := rune(normalized[0])
	if colRune < 'A' || int(colRune-'A')+1 >= size.cols {
		return 0, 0, false
	}

	row, err := strconv.Atoi(normalized[1:])
	if err != nil || row < 1 || row >= size.rows {
		return 0, 0, false
	}

	return row, int(colRune-'A') + 1, true
}
