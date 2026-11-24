package main

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCellRef(t *testing.T) {
	tests := []struct {
		name  string
		ref   string
		size  gridSize
		row   int
		col   int
		valid bool
	}{
		{name: "first cell", ref: "A1", size: gridSize{16, 16}, row: 1, col: 1, valid: true},
		{name: "last default cell", ref: "O15", size: gridSize{16, 16}, row: 15, col: 15, valid: true},
		{name: "lowercase with whitespace", ref: " b4 ", size: gridSize{16, 16}, row: 4, col: 2, valid: true},
		{name: "outside columns", ref: "P1", size: gridSize{16, 16}},
		{name: "outside rows", ref: "A16", size: gridSize{16, 16}},
		{name: "smaller grid", ref: "B2", size: gridSize{3, 3}, row: 2, col: 2, valid: true},
		{name: "outside smaller grid", ref: "C2", size: gridSize{3, 3}},
		{name: "last supported column", ref: "Z10", size: gridSize{11, 27}, row: 10, col: 26, valid: true},
		{name: "invalid row", ref: "A0", size: gridSize{16, 16}},
		{name: "missing row", ref: "A", size: gridSize{16, 16}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, valid := parseCellRef(tt.ref, tt.size)
			assert.Equal(t, tt.valid, valid)
			assert.Equal(t, tt.row, row)
			assert.Equal(t, tt.col, col)
		})
	}
}

func TestGridStoreSaveCell(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grid.sqlite")
	store, err := newGridStore(path, gridSize{rows: 8, cols: 12})
	require.NoError(t, err)

	require.NoError(t, store.saveCell(1, 1, "hello"))
	require.NoError(t, store.saveCell(7, 11, "wide"))
	var value string
	require.NoError(t, store.db.QueryRow("SELECT col_1 FROM grid_rows WHERE id = 1").Scan(&value))
	assert.Equal(t, "hello", value)
	require.NoError(t, store.db.QueryRow("SELECT col_11 FROM grid_rows WHERE id = 7").Scan(&value))
	assert.Equal(t, "wide", value)
	require.NoError(t, store.Close())

	store, err = newGridStore(path, gridSize{rows: 3, cols: 3})
	require.NoError(t, err)
	require.NoError(t, store.db.QueryRow("SELECT col_1 FROM grid_rows WHERE id = 1").Scan(&value))
	assert.Equal(t, "hello", value)
	require.NoError(t, store.Close())

	store, err = newGridStore(path, gridSize{rows: 8, cols: 12})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	require.NoError(t, store.db.QueryRow("SELECT col_11 FROM grid_rows WHERE id = 7").Scan(&value))
	assert.Equal(t, "wide", value)

	entries := make([][]*cellEntry, 8)
	for row := range entries {
		entries[row] = make([]*cellEntry, 12)
		for col := range entries[row] {
			entries[row][col] = &cellEntry{address: fmt.Sprintf("%c%d", 'A'+col-1, row)}
		}
	}
	require.NoError(t, store.load(entries))
	assert.Equal(t, "hello", entries[1][1].raw)
	assert.Equal(t, "wide", entries[7][11].raw)

	entries[7][11].raw = "=A1"
	assert.Equal(t, "hello", evaluateCell(entries, 7, 11, make(map[string]string), make(map[string]bool)))
	entries[7][11].raw = "=L1"
	assert.Equal(t, "#REF!", evaluateCell(entries, 7, 11, make(map[string]string), make(map[string]bool)))
}

func TestGridStoreMigratesLegacyColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grid.sqlite")
	legacy, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = legacy.Exec(`
		CREATE TABLE grid_rows (
			id INTEGER PRIMARY KEY,
			one TEXT NOT NULL DEFAULT '', two TEXT NOT NULL DEFAULT '',
			three TEXT NOT NULL DEFAULT '', four TEXT NOT NULL DEFAULT '',
			five TEXT NOT NULL DEFAULT '', six TEXT NOT NULL DEFAULT '',
			seven TEXT NOT NULL DEFAULT '', eight TEXT NOT NULL DEFAULT '',
			nine TEXT NOT NULL DEFAULT '', ten TEXT NOT NULL DEFAULT '',
			col_11 TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO grid_rows (id, one, ten, col_11) VALUES (7, 'first', 'tenth', 'eleventh');
	`)
	require.NoError(t, err)
	require.NoError(t, legacy.Close())

	store, err := newGridStore(path, gridSize{rows: 3, cols: 3})
	require.NoError(t, err)
	require.NoError(t, store.Close())

	store, err = newGridStore(path, gridSize{rows: 8, cols: 12})
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })
	var first, tenth, eleventh string
	require.NoError(t, store.db.QueryRow("SELECT col_1, col_10, col_11 FROM grid_rows WHERE id = 7").Scan(&first, &tenth, &eleventh))
	assert.Equal(t, "first", first)
	assert.Equal(t, "tenth", tenth)
	assert.Equal(t, "eleventh", eleventh)

	var legacyColumns int
	require.NoError(t, store.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('grid_rows') WHERE name IN ('one', 'ten')").Scan(&legacyColumns))
	assert.Zero(t, legacyColumns)
}
