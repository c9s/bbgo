### Supertrend Strategy

This strategy uses Supertrend indicator as trend, and DEMA indicator as noise filter.
Supertrend strategy needs margin enabled in order to submit short orders, and you can use `leverage` parameter to limit your risk.
**Please note, using leverage higher than 1 is highly risky.**


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
- `takeProfitMultiplier`
    - TP according to ATR multiple, 0 to disable this.
- `stopLossByTriggeringK`
    - Set SL price to the low of the triggering Kline.
- `tpslBySignal`
    - TP/SL by reversed signals.


#### Examples

See [supertrend.yaml](../../config/supertrend.yaml)