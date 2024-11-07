package types

type MarketType string

const (
	MarketTypeSpot    MarketType = "spot"
	MarketTypeFutures MarketType = "futures"
)

type MarketDataType string

const (
	MarketDataTypeTrades    MarketDataType = "trades"
	MarketDataTypeAggTrades MarketDataType = "aggTrades"
	// TODO: could be extended to the following:

	// LEVEL2 = 2
	// https://data.binance.vision/data/futures/um/daily/bookTicker/ADAUSDT/ADAUSDT-bookTicker-2023-11-19.zip
	// update_id	best_bid_price	best_bid_qty	best_ask_price	best_ask_qty	transaction_time	event_time
	// 3.52214E+12	0.3772	        1632	        0.3773	        67521	        1.70035E+12	        1.70035E+12

	// METRICS = 3
	// https://data.binance.vision/data/futures/um/daily/metrics/ADAUSDT/ADAUSDT-metrics-2023-11-19.zip
	// 	create_time	        symbol	sum_open_interest	sum_open_interest_value	count_toptrader_long_short_ratio	sum_toptrader_long_short_ratio	count_long_short_ratio	sum_taker_long_short_vol_ratio
	//  19/11/2023 00:00	ADAUSDT	141979878.00000000	53563193.89339590	    2.33412322	                        1.21401178	                    2.46604727	            0.55265805

	// KLINES    MarketDataType = 4
	// https://public.bybit.com/kline_for_metatrader4/BNBUSDT/2021/BNBUSDT_15_2021-07-01_2021-07-31.csv.gz
	// only few symbols but supported interval options 1m/ 5m/ 15m/ 30m/ 60m/ and only monthly

	// https://data.binance.vision/data/futures/um/daily/klines/1INCHBTC/30m/1INCHBTC-30m-2023-11-18.zip
	// supported interval options 1s/ 1m/ 3m/ 5m/ 15m/ 30m/ 1h/ 2h/ 4h/ 6h/ 8h/ 12h/ 1d/ daily or monthly futures

	// this might be useful for backtesting against mark or index price
	// especially index price can be used across exchanges
	// https://data.binance.vision/data/futures/um/daily/indexPriceKlines/ADAUSDT/1h/ADAUSDT-1h-2023-11-19.zip
	// https://data.binance.vision/data/futures/um/daily/markPriceKlines/ADAUSDT/1h/ADAUSDT-1h-2023-11-19.zip

	// OKex or Bybit do not support direct kLine, metrics or level2 csv download
)
