### Grid Strategy

This strategy places buy and sell orders within the specified price range. The gap between orders are equal, thus they
form `grids`. The price gap is calculated from price range and the number of grids. 


#### Parameters

- `symbol`
    - The trading pair symbol, e.g., `BTCUSDT`, `ETHUSDT`
- `quantity`
    - Quantity per order
- `gridNumber`
    - Number of grids, which is the maximum numbers of orders minus one.
- `profitSpread`
    - The arbitrage profit amount of a set of buy and sell orders. In other words, the profit you want to add to your
      sell order when your buy order is executed.  
- `upperPrice`
    - The upper bond price
- `lowerPrice`
    - The lower bond price
- `long`
    - If true, the sell order is submitted in the same order amount as the filled corresponding buy order, rather than
      the same quantity, which means the arbitrage profit is accumulated in the base asset rather than the quote asset. 
- `catchUp`
    - If true, BBGO will try to submit orders for missing grids. 


#### Examples

See [grid.yaml](../../config/grid.yaml)