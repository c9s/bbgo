package cmd

import (
	"context"
	"os"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

func inBaseAsset(balances types.BalanceMap, market types.Market, price float64) float64 {
	quote := balances[market.QuoteCurrency]
	base := balances[market.BaseCurrency]
	return (base.Locked.Float64() + base.Available.Float64()) + ((quote.Locked.Float64() + quote.Available.Float64()) / price)
}

// configureDB configures the database service based on the environment variable
func configureDB(ctx context.Context, environ *bbgo.Environment) error {
	if driver, ok := os.LookupEnv("DB_DRIVER"); ok {

		if dsn, ok := os.LookupEnv("DB_DSN"); ok {
			return environ.ConfigureDatabase(ctx, driver, dsn)
		}

	} else if dsn, ok := os.LookupEnv("SQLITE3_DSN"); ok {

		return environ.ConfigureDatabase(ctx, "sqlite3", dsn)

	} else if dsn, ok := os.LookupEnv("MYSQL_URL"); ok {

		return environ.ConfigureDatabase(ctx, "mysql", dsn)

	}

	return nil
}
