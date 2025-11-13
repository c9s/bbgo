package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixProfitsUniqueKeyTradeId, down_main_fixProfitsUniqueKeyTradeId)
}

func up_main_fixProfitsUniqueKeyTradeId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS `profits_trade_id`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `profits_trade_id` ON `profits` (`exchange`, `symbol`, `side`, `trade_id`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixProfitsUniqueKeyTradeId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS `profits_trade_id`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `profits_trade_id` ON `profits` (`trade_id`);")
	if err != nil {
		return err
	}
	return err
}
