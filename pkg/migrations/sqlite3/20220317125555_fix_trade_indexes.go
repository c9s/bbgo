package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(upFixTradeIndexes, downFixTradeIndexes)

}

func upFixTradeIndexes(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_symbol;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_symbol_fee_currency;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_traded_at_symbol;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_traded_at ON trades (traded_at, symbol, exchange, id, fee_currency, fee);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_id_traded_at ON trades (id, traded_at);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_order_id_traded_at ON trades (order_id, traded_at);")
	if err != nil {
		return err
	}

	return err
}

func downFixTradeIndexes(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_traded_at;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_id_traded_at;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX IF EXISTS trades_order_id_traded_at;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol ON trades (exchange, symbol);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol_fee_currency ON trades (exchange, symbol, fee_currency, traded_at);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_traded_at_symbol ON trades (exchange, traded_at, symbol);")
	if err != nil {
		return err
	}

	return err
}
