-- @package xfundingv2
-- +up
-- +begin
CREATE TABLE `xfundingv2_closed_rounds`
(
    `gid`                     BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,

    `id`                      VARCHAR(36)        NOT NULL DEFAULT '',
    `strategy_instance_id`    VARCHAR(64)        NOT NULL,
    `spot_symbol`             VARCHAR(32)        NOT NULL,
    `futures_symbol`          VARCHAR(32)        NOT NULL,
    `spot_exchange`           VARCHAR(24)        NOT NULL DEFAULT '',
    `futures_exchange`        VARCHAR(24)        NOT NULL DEFAULT '',
    `direction`               VARCHAR(10)        NOT NULL DEFAULT '',
    `collateral_asset`        VARCHAR(20)        NOT NULL DEFAULT '',

    `leverage`                INT                NOT NULL DEFAULT 0,
    `triggered_funding_rate`  FLOAT(8)           NOT NULL DEFAULT 0,
    `annualized_rate`         FLOAT(8)           NOT NULL DEFAULT 0,
    `funding_income`          DECIMAL(20, 8)     NOT NULL DEFAULT 0,

    `spot_pnl`                DECIMAL(20, 8)     NOT NULL DEFAULT 0,
    `spot_net_pnl`            DECIMAL(20, 8)     NOT NULL DEFAULT 0,
    `futures_pnl`             DECIMAL(20, 8)     NOT NULL DEFAULT 0,
    `futures_net_pnl`         DECIMAL(20, 8)     NOT NULL DEFAULT 0,
    `net_pnl`                 DECIMAL(20, 8)     NOT NULL DEFAULT 0,

    `num_holding_intervals`   INT                NOT NULL DEFAULT 0,

    `start_at`              DATETIME(3)        NOT NULL,
    -- a round is closed before it ever became ready, `ready_at` can be NULL
    `ready_at`              DATETIME(3)        NULL,
    `closing_at`            DATETIME(3)        NOT NULL,
    `closed_at`             DATETIME(3)        NOT NULL,

    `inserted_at`           DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (`gid`),
    KEY `idx_xfundingv2_closed_rounds_instance` (`strategy_instance_id`),
    KEY `idx_xfundingv2_closed_rounds_round_id` (`id`)
);
-- +end

-- +begin
CREATE TABLE `xfundingv2_funding_fees`
(
    `gid`        BIGINT UNSIGNED    NOT NULL AUTO_INCREMENT,

    `round_id`   VARCHAR(36)        NOT NULL DEFAULT '',
    `asset`      VARCHAR(20)        NOT NULL DEFAULT '',
    `amount`     DECIMAL(20, 8)     NOT NULL DEFAULT 0,
    `txn`        BIGINT             NOT NULL DEFAULT 0,
    `time`       DATETIME(3)        NOT NULL,

    `inserted_at` DATETIME(3)       DEFAULT CURRENT_TIMESTAMP(3) NOT NULL,

    PRIMARY KEY (`gid`),
    KEY `idx_xfundingv2_funding_fees_round_id` (`round_id`)
);
-- +end

-- +down
-- +begin
DROP TABLE IF EXISTS `xfundingv2_funding_fees`;
-- +end

-- +begin
DROP TABLE IF EXISTS `xfundingv2_closed_rounds`;
-- +end
