# Strategy Testing

A pre-built small backtest data mysql database file is located at `data/bbgo_test.sql`, which contains 30days BTCUSDT kline data from binance.

for SQLite, it's `data/bbgo_test.sqlite3`.

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

## Testing with MySQL

To import the SQL file into your MySQL database, you can do:

```shell
mysql -uroot -pYOUR_PASSWORD < data/bbgo_test.sql
```

Setup your database correctly:

```shell
DB_DRIVER=mysql
DB_DSN=root:123123@tcp(127.0.0.1:3306)/bbgo
```

## Testing with SQLite3

Create your own sqlite3 database copy in local:

```shell
cp -v data/bbgo_test.sqlite3 bbgo_test.sqlite3
```

Configure the environment variables to use SQLite3:

```shell
DB_DRIVER="sqlite3"
DB_DSN="bbgo_test.sqlite3"
```
