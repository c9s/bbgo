package service

import (
	"context"
	"testing"

	"github.com/c9s/rockhopper"
	"github.com/stretchr/testify/assert"
)

func prepareDB(t *testing.T) (*rockhopper.DB, error) {
	dialect, err := rockhopper.LoadDialect("sqlite3")
	if !assert.NoError(t, err) {
		return nil, err
	}

	assert.NotNil(t, dialect)

	db, err := rockhopper.Open("sqlite3", dialect, ":memory:")
	if !assert.NoError(t, err) {
		return nil, err
	}

	assert.NotNil(t, db)

	_, err = db.CurrentVersion()
	if !assert.NoError(t, err) {
		return nil, err
	}

	var loader rockhopper.SqlMigrationLoader
	migrations, err := loader.Load("../../migrations/sqlite3")
	if !assert.NoError(t, err) {
		return nil, err
	}

	assert.NotEmpty(t, migrations)

	ctx := context.Background()
	err = rockhopper.Up(ctx, db, migrations, 0, 0)
	assert.NoError(t, err, "should migrate successfully")

	return db, err
}
