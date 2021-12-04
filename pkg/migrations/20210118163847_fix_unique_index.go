package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upFixUniqueIndex, downFixUniqueIndex)
}

func upFixUniqueIndex(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP INDEX `id`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD UNIQUE INDEX `id` (`exchange`,`symbol`, `side`, `id`);")
	if err != nil {
		return err
	}

	return err
}

func downFixUniqueIndex(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` DROP INDEX `id`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `trades` ADD UNIQUE INDEX `id` (`id`);")
	if err != nil {
		return err
	}

	return err
}
