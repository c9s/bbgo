### Marketcap Strategy

This strategy will rebalance your portfolio according to the market capitalization from coinmarketcap.

### Prerequisite 

Setup your `COINMARKETCAP_API_KEY` in your environment variables.

#### Parameters

- `interval`
    - The interval to rebalance your portfolio, e.g., `5m`, `1h`
- `quoteCurrency`
    - The quote currency of your portfolio, e.g., `USDT`, `TWD`.
- `quoteCurrencyWeight`
    - The weight of the quote currency in your portfolio. The rest of the weight will be distributed to other currencies by market capitalization.
- `baseCurrencies`
    - A list of currencies you want to hold in your portfolio.
- `threshold`
    - The threshold of the difference between the current weight and the target weight to trigger rebalancing. For example, if the threshold is `1%` and the current weight of `BTC` is `52%` and the target weight is `50%` then the strategy will sell `BTC` until it reaches `50%`.
- `dryRun`
    - If `true`, then the strategy will not place orders.
- `maxAmount` 
    - The maximum amount of each order in quote currency.

#### Examples

See [marketcap.yaml](../../config/marketcap.yaml)
