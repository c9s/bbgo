package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20261116103442_add_margin_interest_valuations.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20261116103442, "migrations/mysql/20261116103442_add_margin_interest_valuations.sql", true,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `margin_interest_valuations`\n(\n    `gid`               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,\n    `interest_gid`      BIGINT UNSIGNED NOT NULL,\n    `exchange`          VARCHAR(24)     NOT NULL,\n    `session`           VARCHAR(64)     NOT NULL,\n    `time`              DATETIME(3)     NOT NULL,\n    `isolated_symbol`   VARCHAR(24)     NOT NULL DEFAULT '',\n    `paid_asset`        VARCHAR(24)     NOT NULL,\n    -- '' when unknown: a converted row without rawAsset\n    `borrowed_asset`    VARCHAR(24)     NOT NULL DEFAULT '',\n    `interest_type`     VARCHAR(32)     NOT NULL DEFAULT '',\n    `converted`         BOOLEAN         NOT NULL DEFAULT FALSE,\n    -- in borrowed_asset units\n    `principal`         DECIMAL(32, 16) UNSIGNED NOT NULL,\n    -- the exchange's rate, daily on Binance\n    `interest_rate`     DECIMAL(32, 16) UNSIGNED NOT NULL,\n    -- in paid_asset units\n    `interest`          DECIMAL(32, 16) UNSIGNED NOT NULL,\n    -- in borrowed_asset units, estimated on converted rows; NULL when unknown\n    `borrowed_interest` DECIMAL(32, 16) UNSIGNED NULL,\n    `price_symbol`      VARCHAR(24)     NOT NULL DEFAULT '',\n    `price_kline_start` DATETIME(3)     NULL,\n    -- NULL until priced\n    `price_in_usdt`     DECIMAL(32, 16) UNSIGNED NULL,\n    `interest_usdt`     DECIMAL(32, 16) UNSIGNED NULL,\n    -- the borrowed asset priced like the paid one, at the same kline\n    `borrowed_price_symbol`  VARCHAR(24)     NOT NULL DEFAULT '',\n    `borrowed_price_in_usdt` DECIMAL(32, 16) UNSIGNED NULL,\n    `borrowed_interest_usdt` DECIMAL(32, 16) UNSIGNED NULL,\n    `principal_usdt`         DECIMAL(32, 16) UNSIGNED NULL,\n    -- priced, unknown_borrowed, no_market, or kline_missing (retried)\n    `price_status`      VARCHAR(16)     NOT NULL,\n    `updated_at`        DATETIME(3)     NOT NULL,\n    PRIMARY KEY (`gid`),\n    UNIQUE KEY `uidx_margin_interest_valuations_interest` (`interest_gid`),\n    KEY `idx_margin_interest_valuations_session_time` (`exchange`, `session`, `time`),\n    KEY `idx_margin_interest_valuations_session_borrowed_time` (`exchange`, `session`, `borrowed_asset`, `time`)\n);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE IF EXISTS `margin_interest_valuations`;"},
		},
	)
}
