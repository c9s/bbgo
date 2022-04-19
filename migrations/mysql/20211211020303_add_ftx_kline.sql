-- +up
-- +begin
create table if not exists ftx_klines
(
    gid bigint unsigned auto_increment
    primary key,
    exchange varchar(10) not null,
    start_time datetime(3) not null,
    end_time datetime(3) not null,
    `interval` varchar(3) not null,
    symbol varchar(20) not null,
    open decimal(20,8) unsigned not null,
    high decimal(20,8) unsigned not null,
    low decimal(20,8) unsigned not null,
    close decimal(20,8) unsigned default 0.00000000 not null,
    volume decimal(20,8) unsigned default 0.00000000 not null,
    closed tinyint(1) default 1 not null,
    last_trade_id int unsigned default '0' not null,
    num_trades int unsigned default '0' not null,
    quote_volume decimal(32,4) default 0.0000 not null,
    taker_buy_base_volume decimal(32,8) not null,
    taker_buy_quote_volume decimal(32,4) default 0.0000 not null
    );
-- +end
-- +begin
create index klines_end_time_symbol_interval
    on ftx_klines (end_time, symbol, `interval`);
-- +end
-- +down

-- +begin
drop table ftx_klines;
-- +end
