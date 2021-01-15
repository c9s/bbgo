package bbgo

import (
	"context"
	"database/sql"

	// register the go migrations
	_ "github.com/c9s/bbgo/pkg/migrations"

	"github.com/c9s/rockhopper"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

func ConnectMySQL(dsn string) (*sqlx.DB, error) {
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}

	config.ParseTime = true
	dsn = config.FormatDSN()
	return sqlx.Connect("mysql", dsn)
}

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
