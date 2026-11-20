//go:build cgo

package fixedpoint

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueScanSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	for _, tc := range []struct {
		name  string
		input any
		want  string
	}{
		{"text price", "0.12345678", "0.12345678"},
		{"negative text amount", "-42.125", "-42.125"},
		{"scientific text amount", "1e-3", "0.001"},
		{"byte price", []byte("0.12345678"), "0.12345678"},
		{"integer amount", int64(42), "42"},
		{"floating amount", float64(42.125), "42.125"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value Value
			require.NoError(t, db.QueryRow("SELECT ?", tc.input).Scan(&value))
			assert.Equal(t, MustNewFromString(tc.want), value)
		})
	}
	t.Run("invalid text returns a parsing error", func(t *testing.T) {
		var value Value
		assert.Error(t, db.QueryRow("SELECT ?", "not-a-number").Scan(&value))
	})
}
