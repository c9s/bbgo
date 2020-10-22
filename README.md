# bbgo

A trading bot framework written in Go. The name bbgo comes from the BB8 bot in the Star Wars movie. aka Buy BitCoin Go!

## Current Status

_Working hard in progress_

[![Build Status](https://travis-ci.org/c9s/bbgo.svg?branch=main)](https://travis-ci.org/c9s/bbgo)

Aim to release v1.0 before 11/14

## Features

- Exchange abstraction interface
- Stream integration (user data websocket)
- PnL calculation.
- Slack notification

## Supported Exchanges

- MAX Exchange (located in Taiwan)
- Binance Exchange

## Installation

Install the builtin commands:

```sh
go get -u github.com/c9s/bbgo/cmd/bbgo
```

Add your dotenv file:

```
SLACK_TOKEN=

BINANCE_API_KEY=
BINANCE_API_SECRET=

MAX_API_KEY=
MAX_API_SECRET=

MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USERNAME=root
MYSQL_PASSWORD=
MYSQL_DATABASE=bbgo
MYSQL_URL=root@tcp(127.0.0.1:3306)/bbgo
```

You can get your API key and secret after you register the accounts:

- For MAX: <https://max.maicoin.com/signup?r=c7982718>
- For Binance: <https://www.binancezh.com/en/register?ref=VGDGLT80>

Then run the `migrate` command to initialize your database:

```sh
dotenv -f .env.local -- bbgo migrate up
```

There are some other commands you can run:

```sh
dotenv -f .env.local -- bbgo migrate status
dotenv -f .env.local -- bbgo migrate redo
```

(It internally uses `goose` to run these migration files, see [migrations](migrations))


To query transfer history:

```sh
dotenv -f .env.local -- bbgo transfer-history --exchange max --asset USDT --since "2019-01-01"
```

To calculate pnl:

```sh
dotenv -f .env.local -- bbgo pnl --exchange binance --asset BTC --since "2019-01-01"
```


## Exchange API Examples

Please check out the example directory: [examples](examples)

Initialize MAX API:

```go
key := os.Getenv("MAX_API_KEY")
secret := os.Getenv("MAX_API_SECRET")

maxRest := maxapi.NewRestClient(maxapi.ProductionAPIURL)
maxRest.Auth(key, secret)
```

Creating user data stream to get the orderbook (depth):

```go
stream := max.NewStream(key, secret)
stream.Subscribe(types.BookChannel, symbol, types.SubscribeOptions{})

streambook := types.NewStreamBook(symbol)
streambook.BindStream(stream)
```

## Strategies

Please check out the directory: [strategies](strategies)

## New API Design

_**still under construction**_

```go
package main

import (
    "github.com/c9s/bbgo"
)

func main() {
    mysqlURL := viper.GetString("mysql-url")
    mysqlURL = fmt.Sprintf("%s?parseTime=true", mysqlURL)
    db, err := sqlx.Connect("mysql", mysqlURL)
    if err != nil {
        return err
    }

    environment := bbgo.NewEnvironment(db)
    environment.AddExchange("binance", binance.New(viper.Getenv("binance-api-key"), viper.Getenv("binance-api-secret"))))
    environment.AddExchange("max", max.New(viper.Getenv("max-key"), viper.Getenv("max-secret"))))

    trader := bbgo.NewTrader(bbgo.Config{
        Environment: environment,
        DB: db,
    })

    trader.AddNotifier(slacknotifier.New(slackToken))
    trader.AddLogHook(slacklog.NewLogHook(slackToken))

    // when any trade execution happened
    trader.OnTrade(func(session string, exchange types.Exchange, trade types.Trade) {
        notify(trade)
        notifyPnL()
    })

    // mount strategy on an exchange
    trader.AddExchangeStrategy("binance",
        bondtrade.New("btcusdt", "5m"),
        bondtrade.New("ethusdt", "5m"))

    // mount cross exchange strategy
    trader.AddCrossExchangeStrategy(hedgemaker.New("max", "binance"))

    t.Run(ctx)
}
```

## Support

### By contributing pull requests

Any pull request is welcome, documentation, format fixing, testing, features.

### By registering account with referral ID

You may register your exchange account with my referral ID to support this project.

- For MAX Exchange: <https://max.maicoin.com/signup?r=c7982718> (default commission rate to your account)
- For Binance Exchange: <https://www.binancezh.com/en/register?ref=VGDGLT80> (5% commission back to your account)

### By small amount cryptos

- BTC address `3J6XQJNWT56amqz9Hz2BEVQ7W4aNmb5kiU`
- USDT ERC20 address `0x63E5805e027548A384c57E20141f6778591Bac6F`

## License

MIT License
