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
- `movingAverageWindow`
    - The MA window to filter out noises, e.g., 99. The support higher than the MA is seen as invalid
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


#### Examples

See [support.yaml](../../config/support.yaml)