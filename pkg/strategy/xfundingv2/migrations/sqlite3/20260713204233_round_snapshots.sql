-- @package xfundingv2
-- +up
-- +begin
CREATE TABLE `xfundingv2_round_snapshots`
(
    `gid`                          INTEGER PRIMARY KEY AUTOINCREMENT,

    `id`                           TEXT     NOT NULL,
    `strategy_instance_id`         TEXT     NOT NULL DEFAULT '',

    `spot_symbol`                  TEXT     NOT NULL,
    `futures_symbol`               TEXT     NOT NULL,

    `spot_exchange`                TEXT     NOT NULL DEFAULT '',
    `futures_exchange`             TEXT     NOT NULL DEFAULT '',

    `direction`                    TEXT     NOT NULL DEFAULT '',
    `collateral_asset`             TEXT     NOT NULL DEFAULT '',

    `leverage`                     REAL     NOT NULL DEFAULT 0,
    `triggered_funding_rate`       REAL     NOT NULL DEFAULT 0,
    `annualized_rate`              REAL     NOT NULL DEFAULT 0,
    `funding_income`               REAL     NOT NULL DEFAULT 0,

    `spot_position`                REAL     NOT NULL DEFAULT 0,
    `futures_position`             REAL     NOT NULL DEFAULT 0,
    `state`                        TEXT     NOT NULL DEFAULT '',

    `spot_price`                   REAL     NOT NULL DEFAULT 0,
    `futures_price`                REAL     NOT NULL DEFAULT 0,
    `spot_average_cost`            REAL     NOT NULL DEFAULT 0,
    `futures_average_cost`         REAL     NOT NULL DEFAULT 0,

    `spot_pnl`                     REAL     NOT NULL DEFAULT 0,
    `spot_net_pnl`                 REAL     NOT NULL DEFAULT 0,
    `futures_pnl`                  REAL     NOT NULL DEFAULT 0,
    `futures_net_pnl`              REAL     NOT NULL DEFAULT 0,
    `net_pnl`                      REAL     NOT NULL DEFAULT 0,

    `unrealized_spot_pnl`          REAL     NOT NULL DEFAULT 0,
    `unrealized_futures_pnl`       REAL     NOT NULL DEFAULT 0,

    `total_spot_net_pnl`      REAL     NOT NULL DEFAULT 0,
    `total_futures_net_pnl`   REAL     NOT NULL DEFAULT 0,
    `total_net_pnl`           REAL     NOT NULL DEFAULT 0,

    `inserted_at`             DATETIME(3)   DEFAULT CURRENT_TIMESTAMP NOT NULL
);
-- +end

-- +begin
CREATE INDEX `idx_xfundingv2_round_snapshots_instance` ON `xfundingv2_round_snapshots` (`strategy_instance_id`);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `xfundingv2_round_snapshots`;
-- +end
