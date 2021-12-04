package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upAddRewardsTable, downAddRewardsTable)
}

func upAddRewardsTable(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `rewards`\n(\n    `gid`         BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    -- for exchange\n    `exchange`    VARCHAR(24)             NOT NULL DEFAULT '',\n    -- reward record id\n    `uuid`        VARCHAR(32)             NOT NULL,\n    `reward_type` VARCHAR(24)             NOT NULL DEFAULT '',\n    -- currency symbol, BTC, MAX, USDT ... etc\n    `currency`    VARCHAR(5)              NOT NULL,\n    -- the quantity of the rewards\n    `quantity`    DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `state`       VARCHAR(5)              NOT NULL,\n    `created_at`  DATETIME                NOT NULL,\n    `spent`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `note`        TEXT                    NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `uuid` (`exchange`, `uuid`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downAddRewardsTable(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `rewards`;")
	if err != nil {
		return err
	}

	return err
}
