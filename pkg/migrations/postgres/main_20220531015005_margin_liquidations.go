package postgres

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_marginLiquidations, down_main_marginLiquidations)
}

func up_main_marginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE TABLE margin_liquidations\n(\n    gid               BIGSERIAL               NOT NULL,\n    exchange          VARCHAR(24)             NOT NULL DEFAULT '',\n    symbol            VARCHAR(24)             NOT NULL DEFAULT '',\n    order_id          BIGINT                  NOT NULL,\n    is_isolated       BOOLEAN                 NOT NULL DEFAULT false,\n    average_price     NUMERIC(16, 8)          NOT NULL CHECK (average_price >= 0),\n    price             NUMERIC(16, 8)          NOT NULL CHECK (price >= 0),\n    quantity          NUMERIC(16, 8)          NOT NULL CHECK (quantity >= 0),\n    executed_quantity NUMERIC(16, 8)          NOT NULL CHECK (executed_quantity >= 0),\n    side              VARCHAR(5)              NOT NULL DEFAULT '',\n    time_in_force     VARCHAR(5)              NOT NULL DEFAULT '',\n    time              TIMESTAMP(3)            NOT NULL,\n    PRIMARY KEY (gid),\n    UNIQUE (order_id, exchange)\n);")
	if err != nil {
		return err
	}
	return err
}

func down_main_marginLiquidations(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS margin_liquidations;")
	if err != nil {
		return err
	}
	return err
}
