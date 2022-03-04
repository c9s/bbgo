-- +up
CREATE TABLE `profits`
(
    `gid`            INTEGER PRIMARY KEY AUTOINCREMENT,

    `exchange`       VARCHAR(24)             NOT NULL DEFAULT '',

    `symbol`         VARCHAR(8)              NOT NULL,

    `trade_id`       INTEGER                 NOT NULL,

    -- average_cost is the position average cost
    `average_cost`   DECIMAL(16, 8) NOT NULL,

    -- profit is the pnl (profit and loss)
    `profit`         DECIMAL(16, 8) NOT NULL,

    -- price is the price of the trade that makes profit
    `price`          DECIMAL(16, 8) NOT NULL,

    -- quantity is the quantity of the trade that makes profit
    `quantity`       DECIMAL(16, 8) NOT NULL,

    -- quote_quantity is the quote quantity of the trade that makes profit
    `quote_quantity` DECIMAL(16, 8) NOT NULL,

    -- side is the side of the trade that makes profit
    `side`           VARCHAR(4)              NOT NULL DEFAULT '',

    `traded_at`      DATETIME(3)             NOT NULL
);

-- +down
DROP TABLE IF EXISTS `profits`;
