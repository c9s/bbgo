package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixUniqueIndex, downFixUniqueIndex)

}

func upFixUniqueIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `trade_unique_id` ON `trades` (`exchange`,`symbol`, `side`, `id`);")
	if err != nil {
		return err
	}

	return err
}

func downFixUniqueIndex(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS `trade_unique_id`;")
	if err != nil {
		return err
	}

	return err
}
