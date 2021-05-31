package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upAddKlineTakerBuyColumns, downAddKlineTakerBuyColumns)

}

func upAddKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE `binance_klines`\n    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `binance_klines`\n    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `binance_klines`\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `max_klines`\n    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `max_klines`\n    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `max_klines`\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE `okex_klines`\n    ADD COLUMN `quote_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `okex_klines`\n    ADD COLUMN `taker_buy_base_volume` DECIMAL NOT NULL DEFAULT 0.0;\nALTER TABLE `okex_klines`\n    ADD COLUMN `taker_buy_quote_volume` DECIMAL NOT NULL DEFAULT 0.0;")
	if err != nil {
		return err
	}

	return err
}

func downAddKlineTakerBuyColumns(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	return err
}
