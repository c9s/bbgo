package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addKucoinKlines, down_main_addKucoinKlines)

}

func up_main_addKucoinKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE `kucoin_klines` LIKE `binance_klines`;")
	if err != nil {
		return err
	}
	return err
}

func down_main_addKucoinKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE `kucoin_klines`;")
	if err != nil {
		return err
	}
	return err
}
