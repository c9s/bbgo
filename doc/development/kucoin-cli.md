# Kucoin command-line tool

```shell
go run ./examples/kucoin accounts
go run ./examples/kucoin subaccounts
go run ./examples/kucoin symbols
go run ./examples/kucoin tickers
go run ./examples/kucoin tickers BTC-USDT
go run ./examples/kucoin orderbook BTC-USDT 20
go run ./examples/kucoin orderbook BTC-USDT 100

go run ./examples/kucoin orders place --symbol LTC-USDT --price 50 --size 1 --order-type limit --side buy
go run ./examples/kucoin orders --symbol LTC-USDT --status active
go run ./examples/kucoin orders --symbol LTC-USDT --status done
go run ./examples/kucoin orders cancel --order-id 61b48b73b4de3e0001251382

go run ./examples/kucoin fills --symbol LTC-USDT
```
