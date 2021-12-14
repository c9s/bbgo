-- +up
-- +begin
CREATE TABLE `asset_balance_history_detail`
(
    gid    bigint unsigned auto_increment primary key,
    `exchange`          VARCHAR             NOT NULL DEFAULT '',
    `subAccount`          VARCHAR             NOT NULL DEFAULT '',
    time   DATETIME(3)     NOT NULL,
    currency     VARCHAR                                not null,
    balanceInUSD DECIMAL(32, 8) default 0.00000000 not null,
    balanceInBTC decimal(32, 8) default 0.00000000 not null,
    balance    decimal(32, 8) default 0.00000000 not null,
    available  decimal(32, 8) default 0.00000000 not null,
    locked     decimal(32, 8) default 0.00000000 not null
);
-- +end
-- +begin
create index idx_asset_balance_history_detail
    on asset_balance_history_detail (time, currency, exchange);
-- +end

-- +down

-- +begin
drop table asset_balance_history_detail;
-- +end
