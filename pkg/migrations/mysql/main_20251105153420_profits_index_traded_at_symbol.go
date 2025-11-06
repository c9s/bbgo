package mysql

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration("main", up_main_profitsIndexTradedAtSymbol, down_main_profitsIndexTradedAtSymbol)
}

func up_main_profitsIndexTradedAtSymbol(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
	_, err = tx.ExecContext(ctx, "CREATE INDEX idx_profits_traded_at_symbol ON profits (traded_at, symbol);")
	if err != nil {
		return err
	}
	return err
}

func down_main_profitsIndexTradedAtSymbol(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
	_, err = tx.ExecContext(ctx, "DROP INDEX idx_profits_traded_at_symbol ON profits;")
	if err != nil {
		return err
	}
	return err
}
