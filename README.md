# bbgo

A trading bot framework written in Go. The name bbgo comes from the BB8 bot in the Star Wars movie. aka Buy BitCoin Go!

## Current Status

[![Build Status](https://travis-ci.org/c9s/bbgo.svg?branch=main)](https://travis-ci.org/c9s/bbgo)

## Features

- Exchange abstraction interface
- Stream integration (user data websocket)
- PnL calculation.
- Slack notification
- KLine-based Backtest
- Built-in strategies

## Supported Exchanges

- MAX Exchange (located in Taiwan)
- Binance Exchange

## Requirements

Get your exchange API key and secret after you register the accounts:

- For MAX: <https://max.maicoin.com/signup?r=c7982718>
- For Binance: <https://www.binancezh.com/en/register?ref=VGDGLT80>

## Installation

Setup MySQL or [run it in docker](https://hub.docker.com/_/mysql)

Install the builtin commands:

```sh
go get -u github.com/c9s/bbgo/cmd/bbgo
```

Add your dotenv file:

```
SLACK_TOKEN=

TELEGRAM_BOT_TOKEN=
TELEGRAM_BOT_AUTH_TOKEN=

BINANCE_API_KEY=
BINANCE_API_SECRET=

MAX_API_KEY=
MAX_API_SECRET=

MYSQL_HOST=127.0.0.1
MYSQL_PORT=3306
MYSQL_USERNAME=root
MYSQL_PASSWORD=
MYSQL_DATABASE=bbgo
# Make sure the following line is correct so you can migrate successfully
MYSQL_URL=root@tcp(127.0.0.1:3306)/bbgo
```

Make sure you have [dotenv](https://github.com/bkeepers/dotenv). Then run the `migrate` command to initialize your database:

```sh
dotenv -f .env.local -- bbgo migrate up
```

There are some other commands you can run:

```sh
dotenv -f .env.local -- bbgo migrate status
dotenv -f .env.local -- bbgo migrate redo
```

(It internally uses `goose` to run these migration files, see [migrations](migrations))

To sync remote exchange klines data for backtesting:

```sh
dotenv -f .env.local -- bbgo backtest --exchange binance --config config/grid.yaml -v --sync --sync-only --sync-from 2020-01-01
```

To run backtest:

```sh
dotenv -f .env.local -- bbgo backtest --exchange binance --config config/bollgrid.yaml --base-asset-baseline
```


To query transfer history:

```sh
dotenv -f .env.local -- bbgo transfer-history --exchange max --asset USDT --since "2019-01-01"
```

To calculate pnl:

```sh
dotenv -f .env.local -- bbgo pnl --exchange binance --asset BTC --since "2019-01-01"
```

To run strategy:

```sh
dotenv -f .env.local -- bbgo run --config config/buyandhold.yaml
```

## Built-in Strategies

Check out the strategy directory [strategy](pkg/strategy) for all built-in strategies:

- `pricealert` strategy demonstrates how to use the notification system [pricealert](pkg/strategy/pricealert)
- `xpuremaker` strategy demonstrates how to maintain the orderbook and submit maker orders [xpuremaker](pkg/strategy/xpuremaker)
- `buyandhold` strategy demonstrates how to subscribe kline events and submit market order [buyandhold](pkg/strategy/buyandhold)
- `grid` strategy implements a basic grid strategy with the built-in bollinger indicator [grid](pkg/strategy/grid)
- `flashcrash` strategy implements a strategy that catches the flashcrash [flashcrash](pkg/strategy/flashcrash)

To run these built-in strategies, just 
modify the config file to make the configuration suitable for you, for example if you want to run
`buyandhold` strategy:

```sh
vim config/buyandhold.yaml

# run bbgo with the config
dotenv -f .env.local -- bbgo run --config config/buyandhold.yaml
```

## Write your own strategy

Create your go package, and initialize the repository with `go mod` and add bbgo as a dependency:

```
go mod init
go get github.com/c9s/bbgo
```

Write your own strategy in the strategy directory like `pkg/strategy/mystrategy`:

```
mkdir pkg/strategy/mystrategy
vim pkg/strategy/mystrategy/strategy.go
```

You can grab the skeleton strategy from <https://github.com/c9s/bbgo/blob/main/pkg/strategy/skeleton/strategy.go>

Now add your config:

```
mkdir config
(cd config && curl -o bbgo.yaml https://raw.githubusercontent.com/c9s/bbgo/main/config/minimal.yaml)
```

Add your strategy package path to the config file `config/bbgo.yaml`

```yaml
imports:
- github.com/xxx/yyy/pkg/strategy/mystrategy
```

Run `bbgo run` command, bbgo will compile a wrapper binary that imports your strategy:

```sh
dotenv -f .env.local -- bbgo run --config config/bbgo.yaml
```

## Dynamic Injection

In order to minimize the strategy code, bbgo supports dynamic dependency injection.

Before executing your strategy, bbgo injects the components into your strategy object if
it found the embedded field that is using bbgo component. for example:

```go
type Strategy struct {
    *bbgo.Notifiability
}
```

And then, in your code, you can call the methods of Notifiability.

Supported components (single exchange strategy only for now):

- `*bbgo.Notifiability`
- `bbgo.OrderExecutor`


If you have `Symbol string` field in your strategy, your strategy will be detected as a symbol-based strategy,
then the following types could be injected automatically:

- `*bbgo.ExchangeSession`
- `types.Market`

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

## Telegram Integration

- In telegram: @botFather
- /newbot
- input bot display name. ex. `bbgo_bot`
- input bot username. This should be global unique. ex. `PeqFqJxP_bbgo_bot`
- Botfather return bot token. Keep bot token safe
- Set `TELEGRAM_BOT_TOKEN` in `.env.local`
- Set `TELEGRAM_AUTH_TOKEN` in `.env.local`. Generate your own auth token. ex. 92463901, or kx2UX@eM
- Run bbgo
- In telegram: search your bot `PeqFqJxP_bbgo_bot`
- /start
- /auth 92463901
- done! your session will route to telegram

## Helm Chart

Prepare your secret:

```
kubectl create secret generic bbgo --from-env-file .env.local
```

Configure your config file, the chart defaults to read config/bbgo.yaml:

```
cp config/grid.yaml config/bbgo.yaml
vim config/bbgo.yaml
```

Install chart:

```
helm install bbgo ./charts/bbgo
```

Delete chart:

```
helm delete bbgo
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
