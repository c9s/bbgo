package sqlite3

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_tradesIndexFix, down_main_tradesIndexFix)

}

func up_main_tradesIndexFix(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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

func down_main_tradesIndexFix(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
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
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol ON trades (symbol);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol_fee_currency ON trades (symbol, fee_currency, traded_at);")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_traded_at_symbol ON trades (traded_at, symbol);")
	if err != nil {
		return err
	}
	return err
}
