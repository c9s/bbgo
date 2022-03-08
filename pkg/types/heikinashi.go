package types

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var Four = fixedpoint.NewFromInt(4)

type HeikinAshi struct {
    GID      uint64       `json:"gid" db:"gid"`
    Exchange ExchangeName `json:"exchange" db:"exchange"`

    Symbol string `json:"symbol" db:"symbol"`

    StartTime Time `json:"startTime" db:"start_time"`
    EndTime   Time `json:"endTime" db:"end_time"`

    Interval Interval `json:"interval" db:"interval"`

    Open                     fixedpoint.Value `json:"open" db:"open"`
    Close                    fixedpoint.Value `json:"close" db:"close"`
    High                     fixedpoint.Value `json:"high" db:"high"`
    Low                      fixedpoint.Value `json:"low" db:"low"`
    Volume                   fixedpoint.Value `json:"volume" db:"volume"`
    QuoteVolume              fixedpoint.Value `json:"quoteVolume" db:"quote_volume"`
    TakerBuyBaseAssetVolume  fixedpoint.Value `json:"takerBuyBaseAssetVolume" db:"taker_buy_base_volume
"`
    TakerBuyQuoteAssetVolume fixedpoint.Value `json:"takerBuyQuoteAssetVolume" db:"taker_buy_quote_volu
me"`

    LastTradeID    uint64 `json:"lastTradeID" db:"last_trade_id"`
    NumberOfTrades uint64 `json:"numberOfTrades" db:"num_trades"`
    Closed         bool   `json:"closed" db:"closed"`
}
