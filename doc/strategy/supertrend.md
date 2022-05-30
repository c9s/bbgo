### Supertrend Strategy

This strategy uses Supertrend indicator as trend, and DEMA indicator as noise filter.
This strategy needs margin enabled in order to submit short orders, but you can use `leverage` parameter to limit your risk.


#### Parameters

- `symbol`
    - The trading pair symbol, e.g., `BTCUSDT`, `ETHUSDT`
- `interval`
    - The K-line interval, e.g., `5m`, `1h`
- `leverage`
    - The leverage of the orders.  
- `fastDEMAWindow`
    - The MA window of the fast DEMA.
- `slowDEMAWindow`
    - The MA window of the slow DEMA.
- `superTrend`
    - Supertrend indicator for deciding current trend.
    - `averageTrueRangeWindow`
        - The MA window of the ATR indicator used by Supertrend. 
    - `averageTrueRangeMultiplier`
        - Multiplier for calculating upper and lower bond prices, the higher, the stronger the trends are, but also makes it less sensitive.


#### Examples

See [supertrend.yaml](../../config/supertrend.yaml)