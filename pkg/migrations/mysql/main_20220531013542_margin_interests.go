package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20220531013542_margin_interests.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20220531013542, "migrations/mysql/20220531013542_margin_interests.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_interests`\n(\n    `gid`             BIGINT UNSIGNED          NOT NULL AUTO_INCREMENT,\n    `exchange`        VARCHAR(24)              NOT NULL DEFAULT '',\n    `asset`           VARCHAR(24)              NOT NULL DEFAULT '',\n    `isolated_symbol` VARCHAR(24)              NOT NULL DEFAULT '',\n    `principle`       DECIMAL(16, 8) UNSIGNED  NOT NULL,\n    `interest`        DECIMAL(20, 16) UNSIGNED NOT NULL,\n    `interest_rate`   DECIMAL(20, 16) UNSIGNED NOT NULL,\n    `time`            DATETIME(3)              NOT NULL,\n    PRIMARY KEY (`gid`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_interests`;"},
		},
	)
}
