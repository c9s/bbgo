### Support Strategy

This strategy uses K-lines with high volume as support and buys the target asset, then takes profit at specified price.


#### Parameters

- `symbol`
    - The trading pair symbol, e.g., `BTCUSDT`, `ETHUSDT`
- `quantity`
    - Quantity per order
- `interval`
    - The K-line interval, e.g., `5m`, `1h`
- `minVolume`
    - The threshold, e.g., `1000000`, `5000000`. A K-line with volume larger than this is seen as a support, and 
      triggers a market buy order.  
- `triggerMovingAverage`
    - The MA window in the current K-line interval to filter out noises. The closed price must be below this MA to 
      trigger the buy order.
    - `interval`
        - The K-line interval, e.g., `5m`, `1h`
    - `window`
        - The MA window in the specified K-line interval to filter out noises.
- `longTermMovingAverage`
    - The MA window in a longer K-line interval. The closed price must be above this MA to trigger the buy order.
    - `interval`
        - The K-line interval, e.g., `5m`, `1h`
    - `window`
        - The MA window in the specified K-line interval to filter out noises.
- `maxBaseAssetBalance`
    - Maximum quantity of the target asset. Orders will not be submitted if the current balance reaches this threshold.
- `minQuoteAssetBalance`
    - Minimum quantity of the quote asset. Orders will not be submitted if the current balance reaches this threshold.
- `targets`
    - `profitPercentage`
        - Take profit ratio, e.g., 0.01 means taking profit when the price rises 1%. 
    - `quantityPercentage`
        - The position ratio to take profit, e.g., 0.5 means selling 50% of the original buy order position when takes 
          profit.
- `trailingStopTarget`
  - Use trailing stop to take profit
  - `callbackRatio`
    - Callback ratio of the trailing stop
  - `minimumProfitPercentage`
    - The minimum profit ratio of the trailing stop. The trailing stop is triggered when the profit is higher than the minimum.


#### Examples

See [support.yaml](../../config/support.yaml)