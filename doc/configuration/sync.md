# Sync Private Trading Data

You can use the following configuration (add this to your bbgo.yaml) to sync your private trading data, like closed
orders and trades:

```yaml
sync:
  # since is the date you want to start sync
  since: 2019-11-01

  # if you have multiple sessions defined, but you don't want to sync all sessions, you can define a list here
  sessions:
  - binance
  - max

  # optional, if you want to insert the trades and orders from the websocket stream
  # if you're running multiple bbgo instance, you should avoid setting this on
  userDataStream:
    # if you set this, all received trades will be written into the database
    trades: true
    # if you set this, all received filled orders will be written into the database
    filledOrders: true

  # symbols is the symbol you want to sync
  # If not defined, BBGO will try to guess your symbols by your existing account balances
  symbols:
  - BTCUSDT
  - ETHUSDT
  - LINKUSDT
```

Then you can start syncing by running the following command:

```shell
bbgo sync --config config/bbgo.yaml
```

Or just (if you have bbgo.yaml is the current working directory):

```shell
bbgo sync
```
