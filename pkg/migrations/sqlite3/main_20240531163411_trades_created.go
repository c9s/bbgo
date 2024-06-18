package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesCreated, down_main_tradesCreated)
}

func up_main_tradesCreated(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE trades ADD COLUMN inserted_at TEXT;\nUPDATE trades SET inserted_at = traded_at;\nCREATE TRIGGER set_inserted_at\nAFTER INSERT ON trades\nFOR EACH ROW\nBEGIN\n    UPDATE trades\n    SET inserted_at = datetime('now')\n    WHERE rowid = NEW.rowid;\nEND;")
	if err != nil {
		return err
	}
	return err
}

func down_main_tradesCreated(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TRIGGER set_inserted_at;\nALTER TABLE trades DROP COLUMN inserted_at;")
	if err != nil {
		return err
	}
	return err
}
