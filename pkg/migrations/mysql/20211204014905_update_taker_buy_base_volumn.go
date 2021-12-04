package mysql

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upUpdateTakerBuyBaseVolumn, downUpdateTakerBuyBaseVolumn)

}

func upUpdateTakerBuyBaseVolumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "ALTER TABLE binance_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE max_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE okex_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(32,8) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}

func downUpdateTakerBuyBaseVolumn(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "ALTER TABLE binance_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE max_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "ALTER TABLE okex_klines CHANGE taker_buy_base_volume taker_buy_base_volume decimal(16,8) NOT NULL;")
	if err != nil {
		return err
	}

	return err
}
