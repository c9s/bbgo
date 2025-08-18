# Using Price Solver

Price solver is a tool to calculate the price of a market based on the prices of other markets. It is useful when you
want to calculate the price of a market that is not directly available on the exchange.

## Simple Price Solver

Simple price solver is a price solver that calculates the price of a market based on the prices of other markets.

You may add a field to the struct to store the price solver:

	priceSolver    *pricesolver.SimplePriceSolver

To use the simple price solver, you need to create an instance of `SimplePriceSolver` and bind the market data stream of
the source markets to it.

	s.priceSolver = pricesolver.NewSimplePriceResolver(sourceMarkets)
	s.priceSolver.BindStream(s.sourceSession.MarketDataStream)

To update the price of the target market, you may call the `UpdatePrice` method of the price solver.
    
    s.priceSolver.Update(symbol, price)
