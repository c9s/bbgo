package cmd

import (
	"context"
	"database/sql"

	"github.com/c9s/rockhopper"
)

func upgradeDB(ctx context.Context, driver string, db *sql.DB) error {
	dialect, err := rockhopper.LoadDialect(driver)
	if err != nil {
		return err
	}

	loader := &rockhopper.GoMigrationLoader{}
	migrations, err := loader.Load()
	if err != nil {
		return err
	}

	rh := rockhopper.New(driver, dialect, db)

	currentVersion, err := rh.CurrentVersion()
	if err != nil {
		return err
	}

	if err := rockhopper.Up(ctx, rh, migrations, currentVersion, 0); err != nil {
		return err
	}

	return nil
}
