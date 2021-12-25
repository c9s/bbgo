package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddKucoinKlines, downAddKucoinKlines)

}

func upAddKucoinKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `kucoin_klines` LIKE `binance_klines`;")
	if err != nil {
		return err
	}

	return err
}

func downAddKucoinKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE `kucoin_klines`;")
	if err != nil {
		return err
	}

	return err
}
