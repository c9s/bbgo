package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addBybitKlines, down_main_addBybitKlines)

}

func up_main_addBybitKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `bybit_klines` LIKE `binance_klines`;")
	if err != nil {
		return err
	}
	return err
}

func down_main_addBybitKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE `bybit_klines`;")
	if err != nil {
		return err
	}
	return err
}
