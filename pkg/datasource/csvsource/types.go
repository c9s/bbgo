// Package csvsource reads locally stored trade CSVs and aggregates them into
// klines for the CSV backtest path.
//
// Deprecated: superseded by pkg/marketdata and its binancecsv source, which
// stream the archives Binance actually publishes, detect timestamp precision
// per value, and merge several sources in time order. What remains here is only
// what service.BacktestServiceCSV still needs, and it goes when the backtest
// engine is rewired.
//
// Known defects in the code kept here, documented rather than fixed because it
// is on its way out:
//
//   - CSVTickToKLine returns from inside its interval loop when a candle opens,
//     so with several intervals configured the remaining ones miss that tick.
//   - addMissingKLines never closes the final candle of a series.
//   - CsvTick.ToGlobalTrade writes the venue's is_buyer_maker into
//     types.Trade.IsMaker, which means "my order was the maker" and is
//     meaningless for a public trade.
package csvsource

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// MarketType and DataType describe a csvsource dataset.
//
// Deprecated: superseded by binancecsv.Market and the dataset names in
// pkg/marketdata/sources/binancecsv.
type MarketType string

// Deprecated: see MarketType.
type DataType string

const (
	SPOT      MarketType = "spot"
	FUTURES   MarketType = "futures"
	TRADES    DataType   = "trades"
	AGGTRADES DataType   = "aggTrades"
)

// Deprecated: superseded by binancecsv.Config, which names the archive tree and
// datasets explicitly instead of collapsing them into a market and a
// granularity.
type CsvConfig struct {
	Market      MarketType `json:"market"`
	Granularity DataType   `json:"granularity"`
}

// CsvTick is a trade decoded from the normalized on-disk format the deleted
// downloader used to write.
//
// Deprecated: the market data layer decodes Binance's published archives
// directly, without a lossy normalization step; see
// pkg/marketdata/sources/binancecsv.
type CsvTick struct {
	Exchange        types.ExchangeName `json:"exchange"`
	Market          MarketType         `json:"market"`
	TradeID         uint64             `json:"tradeID"`
	Symbol          string             `json:"symbol"`
	TickDirection   string             `json:"tickDirection"`
	Side            types.SideType     `json:"side"`
	IsBuyerMaker    bool
	Size            fixedpoint.Value           `json:"size"`
	Price           fixedpoint.Value           `json:"price"`
	HomeNotional    fixedpoint.Value           `json:"homeNotional"`
	ForeignNotional fixedpoint.Value           `json:"foreignNotional"`
	Timestamp       types.MillisecondTimestamp `json:"timestamp"`
}

func (c *CsvTick) ToGlobalTrade() (*types.Trade, error) {
	var isFutures bool
	if c.Market == FUTURES {
		isFutures = true
	}
	return &types.Trade{
		ID: c.TradeID,
		// OrderID:    // not applicable
		Exchange:      c.Exchange,
		Price:         c.Price,
		Quantity:      c.Size,
		QuoteQuantity: c.Price.Mul(c.Size),
		Symbol:        c.Symbol,
		Side:          c.Side,
		IsBuyer:       c.Side == types.SideTypeBuy,
		IsMaker:       c.IsBuyerMaker,
		Time:          types.Time(c.Timestamp),
		// Fee:           trade.ExecFee, // info is overwritten by stream?
		// FeeCurrency:   trade.FeeTokenId,
		IsFutures:  isFutures,
		IsMargin:   false,
		IsIsolated: false,
	}, nil
}
