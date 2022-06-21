package mysql

import (
	"testing"

	"github.com/c9s/rockhopper"
	"github.com/stretchr/testify/assert"
)

func TestGetMigrationsMap(t *testing.T) {
	mm := GetMigrationsMap()
	assert.NotEmpty(t, mm)
}

func TestMergeMigrationsMap(t *testing.T) {
	MergeMigrationsMap(map[int64]*rockhopper.Migration{
		2: {},
		3: {},
	})
}
