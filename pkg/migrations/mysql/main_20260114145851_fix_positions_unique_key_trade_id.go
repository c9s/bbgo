package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_fixPositionsUniqueKeyTradeId, down_main_fixPositionsUniqueKeyTradeId)
}

func up_main_fixPositionsUniqueKeyTradeId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `positions` DROP INDEX `trade_id`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `symbol`, `exchange`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_fixPositionsUniqueKeyTradeId(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `positions` DROP INDEX `trade_id`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `positions` ADD UNIQUE KEY `trade_id` (`trade_id`, `side`, `exchange`);")
	if err != nil {
		return err
	}
	return err
}
