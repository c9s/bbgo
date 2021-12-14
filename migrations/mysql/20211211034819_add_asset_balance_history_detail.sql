-- +up
-- +begin
create table asset_balance_history_detail
(
    gid        bigint unsigned auto_increment
        primary key,
    exchange   varchar(30)                                not null,
    subAccount varchar(30)                                not null,
    time       datetime(3)                                not null,
    currency     varchar(12)                                not null,
    balanceInUSD decimal(32, 8) unsigned default 0.00000000 not null,
    balanceInBTC decimal(32, 8) unsigned default 0.00000000 not null,
    balance    decimal(32, 8) unsigned default 0.00000000 not null,
    available  decimal(32, 8) unsigned default 0.00000000 not null,
    locked     decimal(32, 8) unsigned default 0.00000000 not null
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
