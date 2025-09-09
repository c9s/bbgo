# BBGO Core API

## Reading Account Info

You can get account info from `*bbgo.ExchangeSession`, like this:

```go
account := session.GetAccount()
```

GetAccount() is a thread-safe method, you can call it anywhere.

To retrieve an updated account (real-time data from exchange), you can use `UpdateAccount` method:

```go
account, err := session.UpdateAccount(ctx)
if err != nil {
    log.Error("failed to update account: ", err)
    return
}

// use account here
```

## Reading Current Balances

You can get all balances from `*bbgo.ExchangeSession`, like this:

```go
balances := session.GetAccount().Balances()

// to iterate the balances, you can do:
for _, balance := range balances {
    log.Infof("%s: available: %f, locked: %f\n", balance.Currency, balance.Available.Float64(), balance.Locked.Float64())
}
```

To get one specific balance, you can use `GetBalance` method:

```go
balance, ok := session.GetAccount().Balance("BTC")
if ok {
    log.Infof("BTC balance: available: %f, locked: %f\n", balance.Available.Float64(), balance.Locked.Float64())
} else {
    log.Warn("no BTC balance")
}
```

### Quantity Checking

`types.Market` provides the market info, you can use it to check if the quantity is dust or not:

```
if market.IsDustQuantity(quantity, price) {
    m.logger.Infof("skip dust quantity: %s @ price %f", quantity.String(), price.Float64())
    return nil
}
```

`types.Market` is defined in `bbgo/types/market.go`






