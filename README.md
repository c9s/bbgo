# bbgo

A trading bot framework written in Go. The name bbgo comes from the BB8 bot in the Star Wars movie. aka Buy BitCoin Go!

## Current Status

_Working hard in progress_

aims to release v1.0 before 11/14

## Features

- Exchange abstraction interface
- Stream integration (user data websocket)
- PnL calculation.

## Synopsis

_**still under construction**_

```go
import (
    "github.com/c9s/bbgo"
)

mysqlURL := viper.GetString("mysql-url")
mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
db, err := sqlx.Connect("mysql", mysqlURL)

if err != nil {
    return err
}

t := bbgo.New(bbgo.Config{
    DB: db,
})
t.AddNotifier(slacknotifier.New(slackToken))
t.AddLogHook(slacklog.NewLogHook(slackToken))

t.AddExchange("binance", binance.New(viper.Getenv("bn-key"), viper.Getenv("bn-secret")))).
    Subscribe("binance", "btcusdt", "kline@5m", "book", "trade").
    AddStrategy(bondtrade.New, bondtrade.New).
    Symbols("btcusdt", "bnbusdt")

t.AddExchange("max", max.New(viper.Getenv("max-key"), viper.Getenv("max-secret")))).
    Subscribe("max", "btctwd", "kline@5m", "book", "trade").
    AddStrategy(flashdrop.New, bondtrade.New)

t.AddCrossExchangeStrategy(hedgemaker.New(...))
```

## License

MIT License
