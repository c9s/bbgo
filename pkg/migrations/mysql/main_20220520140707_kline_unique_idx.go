package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_klineUniqueIdx, down_main_klineUniqueIdx)
}

func up_main_klineUniqueIdx(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX idx_kline_binance_unique\n    ON binance_klines (`symbol`, `interval`, `start_time`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX idx_kline_max_unique\n    ON max_klines (`symbol`, `interval`, `start_time`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `idx_kline_ftx_unique`\n    ON ftx_klines (`symbol`, `interval`, `start_time`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `idx_kline_kucoin_unique`\n    ON kucoin_klines (`symbol`, `interval`, `start_time`);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE UNIQUE INDEX `idx_kline_okex_unique`\n    ON okex_klines (`symbol`, `interval`, `start_time`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_klineUniqueIdx(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX `idx_kline_ftx_unique` ON `ftx_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX `idx_kline_max_unique` ON `max_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX `idx_kline_binance_unique` ON `binance_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX `idx_kline_kucoin_unique` ON `kucoin_klines`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DROP INDEX `idx_kline_okex_unique` ON `okex_klines`;")
	if err != nil {
		return err
	}
	return err
}
