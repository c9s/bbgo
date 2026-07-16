package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20260716093831_add_funding_fees.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20260716093831, "migrations/mysql/20260716093831_add_funding_fees.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `funding_fees`\n(\n    `gid`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `exchange`    VARCHAR(24)     NOT NULL DEFAULT '',\n    `symbol`      VARCHAR(20)     NOT NULL DEFAULT '',\n    `asset`       VARCHAR(20)     NOT NULL DEFAULT '',\n    `amount`      DECIMAL(20, 8)  NOT NULL DEFAULT 0,\n    `txn`         BIGINT          NOT NULL DEFAULT 0,\n    `time`        DATETIME(3)     NOT NULL,\n    `inserted_at` DATETIME(3)     DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `uidx_exchange_symbol_txn` (`exchange`, `symbol`, `txn`),\n    KEY `idx_funding_fees_exchange_symbol` (`exchange`, `symbol`),\n    KEY `idx_funding_fees_time` (`time`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `funding_fees`;"},
		},
	)
}
