package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20220531012226_margin_loans.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220531012226, "migrations/mysql/20220531012226_margin_loans.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_loans`\n(\n    `gid`             BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `transaction_id`  BIGINT UNSIGNED         NOT NULL,\n    `exchange`        VARCHAR(24)             NOT NULL DEFAULT '',\n    `asset`           VARCHAR(24)             NOT NULL DEFAULT '',\n    `isolated_symbol` VARCHAR(24)             NOT NULL DEFAULT '',\n    -- quantity is the quantity of the trade that makes profit\n    `principle`       DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `time`            DATETIME(3)             NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY (`transaction_id`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_loans`;"},
		},
	)
}
