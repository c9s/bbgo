## Back-testing

*Before you start back-testing, you need to setup [MySQL](../../README.md#configure-mysql-database) or [SQLite3
](../../README.md#configure-sqlite3-database). Using MySQL is highly recommended.*

First, you need to add the back-testing config to your `bbgo.yaml`:

```yaml
backtest:
  # your back-test will start at the 2021-01-10, be sure to sync the data before 2021-01-10 
  # because some indicator like EMA needs more data to calculate the current EMA value.
  startTime: "2021-01-10"

  # your back-test will end at the 2021-01-10
  endTime: "2021-01-21"
  
  # the symbol data that you want to sync and back-test
  symbols:
  - BTCUSDT

  sessions:
  - binance
  
  # feeMode is optional
  # valid values are: quote, native, token
  #   quote: always deduct fee from the quote balance
  #   native: the crypto exchange fee deduction, base fee for buy order, quote fee for sell order.
  #   token: count fee as crypto exchange fee token
  # feeMode: quote
  
  accounts:
    # the initial account balance you want to start with
    binance: # exchange name
      balances:
        BTC: 0.0
        USDT: 10000.0
```

Note on date formats, the following date formats are supported:
* RFC3339, which looks like `2006-01-02T15:04:05Z07:00`
* RFC822, which looks like `02 Jan 06 15:04 MST`
* You can also use `2021-11-26T15:04:56`

And then, you can sync remote exchange k-lines (candle bars) data for back-testing:

```sh
bbgo backtest -v --sync --config config/grid.yaml
```

To customize the sync data range, add `--sync-from`:

```sh
bbgo backtest -v --sync --sync-only --sync-from 2020-11-01 --config config/grid.yaml
```

Note that, you should sync from an earlier date before your startTime because some indicator like EMA needs more data to calculate the current EMA value.
Here we sync one month before `2021-01-10`.

- `--sync` - sync the data to the latest data point before we start the back-test.
- `--sync-only` - only the back-test data syncing will be executed. do not run back-test.
- `--sync-from` - sync the data from a specific endpoint. note that, once you've start the sync, you can not simply add more data before the initial date.
- `-v` - verbose message output
- `--config config/grid.yaml` - use a specific config file instead of the default config file `./bbgo.yaml`

Run back-test:

```sh
bbgo backtest --base-asset-baseline --config config/grid.yaml
```

If you're developing a strategy, you might want to start with a command like this:

```shell
godotenv -f .env.local -- go run ./cmd/bbgo backtest --config config/grid.yaml --base-asset-baseline
```

## See Also

* [apps/backtest-report](../../apps/backtest-report) - BBGO's built-in backtest report viewer

* [MDD](https://www.investopedia.com/terms/m/maximum-drawdown-mdd.asp) - If you want to test the max draw down (MDD) you can adjust the start date to somewhere near 2020-03-12.
