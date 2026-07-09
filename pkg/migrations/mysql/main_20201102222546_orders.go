package mysql

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from migrations/mysql/20201102222546_orders.sql.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration("main", 20201102222546, "migrations/mysql/20201102222546_orders.sql", false,
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionUp, SQL: "CREATE TABLE `orders`\n(\n    `gid`               BIGINT UNSIGNED         NOT NULL AUTO_INCREMENT,\n    `exchange`          VARCHAR(24)             NOT NULL DEFAULT '',\n    -- order_id is the order id returned from the exchange\n    `order_id`          BIGINT UNSIGNED         NOT NULL,\n    `client_order_id`   VARCHAR(122)            NOT NULL DEFAULT '',\n    `order_type`        VARCHAR(16)             NOT NULL,\n    `symbol`            VARCHAR(32)             NOT NULL,\n    `status`            VARCHAR(12)             NOT NULL,\n    `time_in_force`     VARCHAR(4)              NOT NULL,\n    `price`             DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `stop_price`        DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `quantity`          DECIMAL(16, 8) UNSIGNED NOT NULL,\n    `executed_quantity` DECIMAL(16, 8) UNSIGNED NOT NULL DEFAULT 0.0,\n    `side`              VARCHAR(4)              NOT NULL DEFAULT '',\n    `is_working`        BOOL                    NOT NULL DEFAULT FALSE,\n    `created_at`        DATETIME(3)             NOT NULL,\n    `updated_at`        DATETIME(3)             NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),\n    `is_margin`         BOOLEAN                 NOT NULL DEFAULT FALSE,\n    `is_isolated`       BOOLEAN                 NOT NULL DEFAULT FALSE,\n    PRIMARY KEY (`gid`)\n);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE INDEX orders_symbol ON orders (exchange, symbol);"},
			{Direction: rockhopper.DirectionUp, SQL: "CREATE UNIQUE INDEX orders_order_id ON orders (order_id, exchange);"},
		},
		[]rockhopper.Statement{
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX orders_symbol ON orders;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP INDEX orders_order_id ON orders;"},
			{Direction: rockhopper.DirectionDown, SQL: "DROP TABLE `orders`;"},
		},
	)
}
