package migration

import (
	"context"
	"database/sql"
	"log"
)

// UpTo migrates up to a specific version.
func UpTo(ctx context.Context, db *sql.DB, dir string, version int64) error {
	migrations, err := CollectMigrationsFromDir(dir, minVersion, version)
	if err != nil {
		return err
	}

	for {
		current, err := GetDBVersion(db)
		if err != nil {
			return err
		}

		next, err := migrations.Next(current)
		if err != nil {
			if err == ErrNoNextVersion {
				log.Printf("no migrations to run. current version: %d\n", current)
				return nil
			}
			return err
		}

		if err = next.Up(ctx, db); err != nil {
			return err
		}
	}
}

// Up applies all available migrations.
func Up(ctx context.Context, db *sql.DB, dir string) error {
	return UpTo(ctx, db, dir, maxVersion)
}

// UpByOne migrates up by a single version.
func UpByOne(ctx context.Context, db *sql.DB, dir string) error {
	migrations, err := CollectMigrationsFromDir(dir, minVersion, maxVersion)
	if err != nil {
		return err
	}

	currentVersion, err := GetDBVersion(db)
	if err != nil {
		return err
	}

	next, err := migrations.Next(currentVersion)
	if err != nil {
		if err == ErrNoNextVersion {
			log.Printf("no migrations to run. current version: %d\n", currentVersion)
			return nil
		}
		return err
	}

	if err = next.Up(ctx, db); err != nil {
		return err
	}

	return nil
}
