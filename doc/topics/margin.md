# Margin Trading

Margin trading is a method of trading assets using funds provided by a third party. When compared to regular trading accounts, margin accounts allow traders to access greater sums of capital, allowing them to leverage their positions. Essentially, margin trading amplifies trading results so that traders are able to realize larger profits on successful trades. This ability to expand trading results makes margin trading especially popular in low-volatility markets, particularly the international Forex market. Nonetheless, it is important to note that margin trading involves a significant amount of risk. While you can use leverage to make a greater profit, you can also incur significant losses if a trade moves against you.

Currently, bbgo supports cross margin trading and isolated margin trading. Enable margin trading by setting the `margin` field under the session settings in the bbgo.yaml configuration file, for example:
    
```yaml
sessions:
  binance:
    exchange: binance
    envVarPrefix: BINANCE
    margin: true
```

## Margin Trading on OKX

To enable margin trading on OKX, you need to set the trading mode to "Spot Mode",
and turn on Auto-Borrow from the OKX UI.


