package indicator

/*
NEW INDICATOR DESIGN:

klines := kLines(marketDataStream)
closePrices := closePrices(klines)
macd := MACD(klines, {Fast: 12, Slow: 10})

equals to:

klines := KLines(marketDataStream)
closePrices := ClosePrice(klines)
fastEMA := EMA(closePrices, 7)
slowEMA := EMA(closePrices, 25)
macd := Subtract(fastEMA, slowEMA)
signal := EMA(macd, 16)
histogram := Subtract(macd, signal)
*/
