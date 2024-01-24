package sqlite3

import (
	"testing"

	"github.com/c9s/rockhopper/v2"

	"github.com/stretchr/testify/assert"
)

func TestGetMigrationsMap(t *testing.T) {
	mm := GetMigrationsMap()
	assert.NotEmpty(t, mm)
}

func TestMergeMigrationsMap(t *testing.T) {
	MergeMigrationsMap(map[rockhopper.RegistryKey]*rockhopper.Migration{
		rockhopper.RegistryKey{Version: 2}: &rockhopper.Migration{},
		rockhopper.RegistryKey{Version: 2}: &rockhopper.Migration{},
	})
}
