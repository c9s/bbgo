package migrations

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(upTradeIndex, downTradeIndex)
}

func upTradeIndex(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol ON trades(symbol);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_symbol_fee_currency ON trades(symbol, fee_currency, traded_at);")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "CREATE INDEX trades_traded_at_symbol ON trades(traded_at, symbol);")
	if err != nil {
		return err
	}

	return err
}

func downTradeIndex(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.

	_, err = tx.ExecContext(ctx, "DROP INDEX trades_symbol ON trades;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX trades_symbol_fee_currency ON trades;")
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, "DROP INDEX trades_traded_at_symbol ON trades;")
	if err != nil {
		return err
	}

	return err
}
