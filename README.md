# bbgo

A trading bot framework written in Go. The name bbgo comes from the BB8 bot in the Star Wars movie. aka Buy BitCoin Go!

## Current Status

[![Build Status](https://travis-ci.org/c9s/bbgo.svg?branch=main)](https://travis-ci.org/c9s/bbgo)

## Features

- Exchange abstraction interface
- Stream integration (user data websocket)
- PnL calculation
- Slack notification
- KLine-based backtest
- Built-in strategies
- Multi-session support
- Standard indicators (SMA, EMA, BOLL)

## Supported Exchanges

- MAX Exchange (located in Taiwan)
- Binance Exchange

## Requirements

Get your exchange API key and secret after you register the accounts:

- For MAX: <https://max.maicoin.com/signup?r=c7982718>
- For Binance: <https://www.binancezh.com/en/register?ref=VGDGLT80>

## Installation


### Install from binary

The following script will help you setup a config file, dotenv file:

```
bash <(curl -s https://raw.githubusercontent.com/c9s/bbgo/main/scripts/setup-grid.sh)
```

### Install from source

Optional: setup MySQL or [run it in docker](https://hub.docker.com/_/mysql)

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

FTX_API_KEY=
FTX_API_SECRET=
# specify it if credentials are for subaccount
FTX_SUBACCOUNT=

MYSQL_URL=root@tcp(127.0.0.1:3306)/bbgo?parseTime=true
```

Prepare your dotenv file `.env.local` and BBGO yaml config file `bbgo.yaml`.

To sync your own trade data:

```
bbgo sync --session max
bbgo sync --session binance
```

If you want to switch to other dotenv file, you can add an `--dotenv` option or `--config`:

```
bbgo sync --dotenv .env.dev --config config/grid.yaml --session binance
```


To sync remote exchange klines data for backtesting:

```sh
bbgo backtest --exchange binance -v --sync --sync-only --sync-from 2020-01-01
```

To run backtest:

```sh
bbgo backtest --exchange binance --base-asset-baseline
```


To query transfer history:

```sh
bbgo transfer-history --session max --asset USDT --since "2019-01-01"
```

To calculate pnl:

```sh
bbgo pnl --exchange binance --asset BTC --since "2019-01-01"
```

To run strategy:

```sh
bbgo run
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
bbgo run --config config/buyandhold.yaml
```

## Write your own strategy

Create your go package, and initialize the repository with `go mod` and add bbgo as a dependency:

```
go mod init
go get github.com/c9s/bbgo@main
```

Write your own strategy in the strategy file:

```
vim strategy.go
```

You can grab the skeleton strategy from <https://github.com/c9s/bbgo/blob/main/pkg/strategy/skeleton/strategy.go>

Now add your config:

```
mkdir config
(cd config && curl -o bbgo.yaml https://raw.githubusercontent.com/c9s/bbgo/main/config/minimal.yaml)
```

Add your strategy package path to the config file `config/bbgo.yaml`

```yaml
---
build:
  dir: build
  imports:
  - github.com/your_id/your_swing
  targets:
  - name: swing-amd64-linux
    os: linux
    arch: amd64
  - name: swing-amd64-darwin
    os: darwin
    arch: amd64
```

Run `bbgo run` command, bbgo will compile a wrapper binary that imports your strategy:

```sh
dotenv -f .env.local -- bbgo run --config config/bbgo.yaml
```

Or you can build your own wrapper binary via:

```shell
bbgo build --config config/bbgo.yaml
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

Prepare your docker image locally (you can also use the docker image from docker hub):

```
make docker DOCKER_TAG=1.16.0
```

The docker tag version number is from the file [Chart.yaml](charts/bbgo/Chart.yaml)

Prepare your secret:

```
kubectl create secret generic bbgo-grid --from-env-file .env.local
```

Configure your config file, the chart defaults to read config/bbgo.yaml to
create a configmap:

```
cp config/grid.yaml config/bbgo.yaml
vim config/bbgo.yaml
```

Install chart with the preferred release name, the release name maps to the
previous secret we just created, that is, `bbgo-grid`:

```
helm install bbgo-grid ./charts/bbgo
```

Delete chart:

```
helm delete bbgo
```

## Development

### Adding new migration

```sh
rockhopper --config rockhopper_sqlite.yaml create --type sql add_pnl_column
rockhopper --config rockhopper_mysql.yaml create --type sql add_pnl_column
```

or

```
bash utils/generate-new-migration.sh add_pnl_column
```

Be sure to edit both sqlite3 and mysql migration files.

To test the drivers, you can do:

```
rockhopper --config rockhopper_sqlite.yaml up
rockhopper --config rockhopper_mysql.yaml up
```

### Setup frontend development environment

```
cd frontend
yarn install
```

### Testing Desktop App

for webview

```sh
make embed && go run -tags web ./cmd/bbgo-webview
```

for lorca

```sh
make embed && go run -tags web ./cmd/bbgo-lorca
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


## Community

You can join our telegram channel <https://t.me/bbgocrypto>, it's in Chinese, but English is fine as well.

## Contribution

BBGO has a token BBG for the ecosystem (contract address: <https://etherscan.io/address/0x3afe98235d680e8d7a52e1458a59d60f45f935c0>).

Each issue has its BBG label, by completing the issue with a pull request, you can get correspond amount of BBG.

If you have feature request, you can offer your BBG for contributors.

For further request, please contact us: <https://t.me/c123456789s>

## License

MIT License
