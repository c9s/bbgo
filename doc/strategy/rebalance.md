### Rebalance Strategy

This strategy automatically rebalances your portfolio based on target weights.

#### Parameters

- `schedule`
    - The CRON expression used to schedule rebalance executions, for example: `@every 5m`, `@every 1h`, `0 0 * * * *`.
    - [CRON Expression Format](https://pkg.go.dev/github.com/robfig/cron#hdr-CRON_Expression_Format)
- `quoteCurrency`
    - The quote currency in trading markets, e.g., `USDT`, `TWD`.
- `targetWeights`
    - A map defining the desired target weights for each base currency in the portfolio.
- `threshold`
    - The threshold represents the maximum allowable difference between the current weight and the target weight that will initiate rebalancing. For instance, with a threshold set at `1%`, if the current weight of `BTC` is `52%` and the target weight is `50%`, the strategy will sell `BTC` until it reaches `50%`.
- `maxAmount` 
    - The highest allowable value for each order in the quote currency.
- `orderType`
    - The type of order to be placed, e.g., `LIMIT`, `LIMIT_MAKER`, `MARKET`
- `priceType`
    - The method used to determine the price for buying or selling the currency, for example, `MAKER`, `TAKER`, `MID`, `LAST`
- `balanceType`
    - Determines how the current weights are calculated. If set to `TOTAL`, the weights will include locked funds. e.g. `TOTAL`, `AVAILABLE`.
- `dryRun`
    - If set to `true`, the strategy will simulate rebalancing without actually placing orders.
- `onStart`
    - Indicates whether to rebalance the portfolio when the strategy starts.

#### Examples

See [rebalance.yaml](../../config/rebalance.yaml)
