package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCellRef(t *testing.T) {
	tests := []struct {
		name  string
		ref   string
		row   int
		col   int
		valid bool
	}{
		{name: "first cell", ref: "A1", row: 1, col: 1, valid: true},
		{name: "last cell", ref: "I9", row: 9, col: 9, valid: true},
		{name: "lowercase with whitespace", ref: " b4 ", row: 4, col: 2, valid: true},
		{name: "outside grid", ref: "J1"},
		{name: "invalid row", ref: "A0"},
		{name: "missing row", ref: "A"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, col, valid := parseCellRef(tt.ref)
			assert.Equal(t, tt.valid, valid)
			assert.Equal(t, tt.row, row)
			assert.Equal(t, tt.col, col)
		})
	}
}

func TestGridStoreSaveCell(t *testing.T) {
	store, err := newGridStore(filepath.Join(t.TempDir(), "grid.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, store.Close()) })

	require.NoError(t, store.saveCell(1, 1, "hello"))
	var value string
	require.NoError(t, store.db.QueryRow("SELECT one FROM grid_rows WHERE id = 1").Scan(&value))
	assert.Equal(t, "hello", value)
}
