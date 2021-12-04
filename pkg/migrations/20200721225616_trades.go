package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upTrades, downTrades)
}

func upTrades(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE TABLE `trades`\n(\n    `gid`            BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `id`             BIGINT UNSIGNED,\n    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',\n    `symbol`         VARCHAR(8)              NOT NULL,\n    `price`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`       DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quote_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee`            DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `fee_currency`   VARCHAR(4)              NOT NULL,\n    `is_buyer`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_maker`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `side`           VARCHAR(4)              NOT NULL DEFAULT '',\n    `traded_at`      DATETIME(3)             NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `id` (`id`)\n);")
	if err != nil {
		return err
	}

	return err
}

func downTrades(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP TABLE IF EXISTS `trades`;")
	if err != nil {
		return err
	}

	return err
}
