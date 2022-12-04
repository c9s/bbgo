## Strategy Testing

A pre-built small backtest data db file is located at `data/bbgo_test.sql`, which contains 30days BTCUSDT kline data from binance.

You can use this file for environments without networking to test your strategy.

A small backtest data set is synchronized in the database:

- exchange: binance
- symbol: BTCUSDT
- startDate: 2022-06-01
- endDate: 2022-06-30

The SQL file is added via git-lfs, so you need to install git-lfs first:

```shell
git lfs install
```

To import the database, you can do:

```shell
mysql -uroot -pYOUR_PASSWORD < data/bbgo_test.sql
```
