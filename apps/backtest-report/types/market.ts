export interface Market {
  symbol:          string;
  localSymbol:     string;
  pricePrecision:  number;
  volumePrecision: number;
  quoteCurrency:   string;
  baseCurrency:    string;
  minNotional:     number;
  minAmount:       number;
  minQuantity:     number;
  maxQuantity:     number;
  stepSize:        number;
  minPrice:        number;
  maxPrice:        number;
  tickSize:        number;
}
