package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addKlineTakerBuyColumns, down_main_addKlineTakerBuyColumns)
}

func up_main_addKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `klines`\n    ADD COLUMN `quote_volume`           DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_base_volume`  DECIMAL(32, 8) NOT NULL DEFAULT 0.0,\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL(32, 8) NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}
	return err
}

func down_main_addKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\n    DROP COLUMN `quote_volume`,\n    DROP COLUMN `taker_buy_base_volume`,\n    DROP COLUMN `taker_buy_quote_volume`;")
	if err != nil {
		return err
	}
	return err
}
