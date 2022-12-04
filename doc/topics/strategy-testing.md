## Strategy Testing

A pre-built small backtest data db file is located at `data/bbgo_test.sql`.

You can use this file for environments without networking to test your strategy.

A small backtest data set is synchronized in the database:

- exchange: binance
- symbol: BTCUSDT
- startDate: 2022-06-01
- endDate: 2022-06-30

To import the database, you can do:

```shell
mysql -uroot -pYOUR_PASSWORD < data/bbgo_test.sql
```
