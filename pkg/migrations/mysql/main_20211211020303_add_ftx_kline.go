package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_addFtxKline, down_main_addFtxKline)

}

func up_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "create table if not exists ftx_klines\n(\n    gid bigint unsigned auto_increment\n    primary key,\n    exchange varchar(10) not null,\n    start_time datetime(3) not null,\n    end_time datetime(3) not null,\n    `interval` varchar(3) not null,\n    symbol varchar(20) not null,\n    open decimal(20,8) unsigned not null,\n    high decimal(20,8) unsigned not null,\n    low decimal(20,8) unsigned not null,\n    close decimal(20,8) unsigned default 0.00000000 not null,\n    volume decimal(20,8) unsigned default 0.00000000 not null,\n    closed tinyint(1) default 1 not null,\n    last_trade_id int unsigned default '0' not null,\n    num_trades int unsigned default '0' not null,\n    quote_volume decimal(32,4) default 0.0000 not null,\n    taker_buy_base_volume decimal(32,8) not null,\n    taker_buy_quote_volume decimal(32,4) default 0.0000 not null\n    );")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "create index klines_end_time_symbol_interval\n    on ftx_klines (end_time, symbol, `interval`);")
	if err != nil {
		return err
	}
	return err
}

func down_main_addFtxKline(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "drop table ftx_klines;")
	if err != nil {
		return err
	}
	return err
}
