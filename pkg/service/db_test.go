package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/rockhopper/v2"
)

func prepareDB(t *testing.T) (*rockhopper.DB, error) {
	ctx := context.Background()

	dialect, err := rockhopper.LoadDialect("sqlite3")
	if !assert.NoError(t, err) {
		return nil, err
	}

	assert.NotNil(t, dialect)

	db, err := rockhopper.Open("sqlite3", dialect, ":memory:", rockhopper.TableName)
	if !assert.NoError(t, err) {
		return nil, err
	}

	assert.NotNil(t, db)

	err = db.Touch(ctx)
	if !assert.NoError(t, err) {
		return nil, err
	}

	var loader = &rockhopper.SqlMigrationLoader{}

	migrations, err := loader.Load("../../migrations/sqlite3")
	if !assert.NoError(t, err) {
		return nil, err
	}

	migrations = migrations.Sort().Connect()
	assert.NotEmpty(t, migrations)

	err = rockhopper.Up(ctx, db, migrations.Head(), 0)
	assert.NoError(t, err, "should migrate successfully")

	return db, err
}
