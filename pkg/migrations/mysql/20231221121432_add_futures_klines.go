package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddFuturesKlines, downAddFuturesKlines)

}

func upAddFuturesKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `binance_futures_klines` LIKE `binance_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE TABLE `bybit_futures_klines` LIKE `bybit_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE TABLE `okex_futures_klines` LIKE `okex_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE TABLE `max_futures_klines` LIKE `max_klines`;")
	if err != nil {
		return err
	}

	return err
}

func downAddFuturesKlines(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `binance_futures_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `bybit_futures_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `okex_futures_klines`;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `max_futures_klines`;")
	if err != nil {
		return err
	}

	return err
}
