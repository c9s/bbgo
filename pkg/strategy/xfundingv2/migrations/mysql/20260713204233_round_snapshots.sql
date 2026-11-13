-- @package xfundingv2
-- +up
-- +begin
CREATE TABLE `xfundingv2_round_snapshots`
(
    `gid`                          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,

    `id`                           VARCHAR(36)     NOT NULL,
    `strategy_instance_id`         VARCHAR(64)     NOT NULL DEFAULT '',

    `spot_symbol`                  VARCHAR(32)     NOT NULL,
    `futures_symbol`               VARCHAR(32)     NOT NULL,

    `spot_exchange`                VARCHAR(24)     NOT NULL DEFAULT '',
    `futures_exchange`             VARCHAR(24)     NOT NULL DEFAULT '',

    `direction`                    VARCHAR(16)     NOT NULL DEFAULT '',
    `collateral_asset`             VARCHAR(20)     NOT NULL DEFAULT '',

    `leverage`                     DECIMAL(16, 8)  NOT NULL DEFAULT 0,
    `triggered_funding_rate`       DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `annualized_rate`              DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `funding_income`               DECIMAL(20, 8)  NOT NULL DEFAULT 0,

    `spot_position`                DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `futures_position`             DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `state`                        VARCHAR(16)     NOT NULL DEFAULT '',

    `spot_price`                   DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `futures_price`                DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `spot_average_cost`            DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `futures_average_cost`         DECIMAL(20, 8)  NOT NULL DEFAULT 0,

    `spot_pnl`                     DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `spot_net_pnl`                 DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `futures_pnl`                  DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `futures_net_pnl`              DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `net_pnl`                      DECIMAL(20, 8)  NOT NULL DEFAULT 0,

    `unrealized_spot_pnl`          DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `unrealized_futures_pnl`       DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    
    `total_spot_net_pnl`      DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `total_futures_net_pnl`   DECIMAL(20, 8)  NOT NULL DEFAULT 0,
    `total_net_pnl`           DECIMAL(20, 8)  NOT NULL DEFAULT 0,

    `inserted_at`             DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (`gid`),
    KEY `idx_xfundingv2_round_snapshots_instance` (`strategy_instance_id`)
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `xfundingv2_round_snapshots`;
-- +end
