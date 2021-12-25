## TWAP Order Execution

Yes, bbgo supports TWAP order execution. If you have a large quantity order want to execute,
you can use this feature to update your order price according to the first bid/ask price in the order book.


### Usage

```
bbgo execute-order --session binance --symbol=BTCUSDT \
   --side=sell \
   --target-quantity=100.0 \
   --slice-quantity=0.01 \
   --stop-price=58000
```

The above command will sell 100 BTC in total and for each slice it places a limit sell order with 0.01 BTC, and only place the sell order when price is above 58000.

```
bbgo execute-order --session max --symbol=USDTTWD --side=sell --target-quantity=1000.0 --slice-quantity=100.0 --stop-price=28.90
```

`--symbol=SYMBOL` is the symbol of the market, the symbol should be in upper-case, for example, `USDTTWD` or `BTCUSDT`

`--side=SIDE` is the side of your order. can be `buy` or `sell`.

`--target-quantity=TARGET_QUANTITY` is the final quantity you want to buy/sell.

`--slice-quantity=SLICE_QUANTITY` is the slice quantity per order. for example, if you have targetQuantity=100.0 and sliceQuantity=10.0, then the order will be split into 10 orders.

`--stop-price` the highest/lowest price of your order. for example, the current best bid/ask price is 28.65 and 28.70,
if your stop price for BUY is `28.60`, your BUY order will stay at price `28.6` and won't be updated.
if your stop price for SELL is `28.75`, your SELL order will stay at price `28.75` and won't be updated.

`--price-ticks` the incremental tick spread of the price. for example, the current best bid/ask price is 28.00 and 28.10,
the single tick of the USDT/TWD symbol is 0.01,
if you set `--price-ticks=2`, then the order executor will use 28.00 + 0.01 * 2 for your BUY order, and use 28.10 - 0.01 * 2 for your SELL order.

`--deadline` the deadline duration of your order execution, if time exceeded the deadline time, then the rest quantity will be sent as a market order.
