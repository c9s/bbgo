package ftx

import (
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// ex: 2019-03-05T09:56:55.728933+00:00
const timeLayout = "2006-01-02T15:04:05.999999Z07:00"

type datetime struct {
	time.Time
}

func parseDatetime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

// used in unit test
func mustParseDatetime(s string) time.Time {
	t, err := parseDatetime(s)
	if err != nil {
		panic(err)
	}
	return t
}

func (d *datetime) UnmarshalJSON(b []byte) error {
	// remove double quote from json string
	s := strings.Trim(string(b), "\"")
	if len(s) == 0 {
		d.Time = time.Time{}
		return nil
	}
	t, err := parseDatetime(s)
	if err != nil {
		return err
	}
	d.Time = t
	return nil
}

/*
{
  "success": true,
  "result": {
    "backstopProvider": true,
    "collateral": 3568181.02691129,
    "freeCollateral": 1786071.456884368,
    "initialMarginRequirement": 0.12222384240257728,
    "leverage": 10,
    "liquidating": false,
    "maintenanceMarginRequirement": 0.07177992558058484,
    "makerFee": 0.0002,
    "marginFraction": 0.5588433331419503,
    "openMarginFraction": 0.2447194090423075,
    "takerFee": 0.0005,
    "totalAccountValue": 3568180.98341129,
    "totalPositionSize": 6384939.6992,
    "username": "user@domain.com",
    "positions": [
      {
        "cost": -31.7906,
        "entryPrice": 138.22,
        "future": "ETH-PERP",
        "initialMarginRequirement": 0.1,
        "longOrderSize": 1744.55,
        "maintenanceMarginRequirement": 0.04,
        "netSize": -0.23,
        "openSize": 1744.32,
        "realizedPnl": 3.39441714,
        "shortOrderSize": 1732.09,
        "side": "sell",
        "size": 0.23,
        "unrealizedPnl": 0
      }
    ]
  }
}
*/
type accountResponse struct { // nolint:golint,deadcode
	Success bool    `json:"success"`
	Result  account `json:"result"`
}

type account struct {
	MakerFee          fixedpoint.Value `json:"makerFee"`
	TakerFee          fixedpoint.Value `json:"takerFee"`
	TotalAccountValue fixedpoint.Value `json:"totalAccountValue"`
}

type positionsResponse struct { // nolint:golint,deadcode
	Success bool       `json:"success"`
	Result  []position `json:"result"`
}

/*
{
  "cost": -31.7906,
  "entryPrice": 138.22,
  "estimatedLiquidationPrice": 152.1,
  "future": "ETH-PERP",
  "initialMarginRequirement": 0.1,
  "longOrderSize": 1744.55,
  "maintenanceMarginRequirement": 0.04,
  "netSize": -0.23,
  "openSize": 1744.32,
  "realizedPnl": 3.39441714,
  "shortOrderSize": 1732.09,
  "side": "sell",
  "size": 0.23,
  "unrealizedPnl": 0,
  "collateralUsed": 3.17906
}
*/
type position struct {
	Cost                         fixedpoint.Value `json:"cost"`
	EntryPrice                   fixedpoint.Value `json:"entryPrice"`
	EstimatedLiquidationPrice    fixedpoint.Value `json:"estimatedLiquidationPrice"`
	Future                       string           `json:"future"`
	InitialMarginRequirement     fixedpoint.Value `json:"initialMarginRequirement"`
	LongOrderSize                fixedpoint.Value `json:"longOrderSize"`
	MaintenanceMarginRequirement fixedpoint.Value `json:"maintenanceMarginRequirement"`
	NetSize                      fixedpoint.Value `json:"netSize"`
	OpenSize                     fixedpoint.Value `json:"openSize"`
	RealizedPnl                  fixedpoint.Value `json:"realizedPnl"`
	ShortOrderSize               fixedpoint.Value `json:"shortOrderSize"`
	Side                         string           `json:"Side"`
	Size                         fixedpoint.Value `json:"size"`
	UnrealizedPnl                fixedpoint.Value `json:"unrealizedPnl"`
	CollateralUsed               fixedpoint.Value `json:"collateralUsed"`
}

type balances struct { // nolint:golint,deadcode
	Success bool `json:"success"`

	Result []struct {
		Coin  string           `json:"coin"`
		Free  fixedpoint.Value `json:"free"`
		Total fixedpoint.Value `json:"total"`
	} `json:"result"`
}

/*
[
  {
    "name": "BTC/USD",
    "enabled": true,
    "postOnly": false,
    "priceIncrement": 1.0,
    "sizeIncrement": 0.0001,
    "minProvideSize": 0.0001,
    "last": 59039.0,
    "bid": 59038.0,
    "ask": 59040.0,
    "price": 59039.0,
    "type": "spot",
    "baseCurrency": "BTC",
    "quoteCurrency": "USD",
    "underlying": null,
    "restricted": false,
    "highLeverageFeeExempt": true,
    "change1h": 0.0015777151969599294,
    "change24h": 0.05475756601279165,
    "changeBod": -0.0035107262814994852,
    "quoteVolume24h": 316493675.5463,
    "volumeUsd24h": 316493675.5463
  }
]
*/
type marketsResponse struct { // nolint:golint,deadcode
	Success bool     `json:"success"`
	Result  []market `json:"result"`
}

type market struct {
	Name                  string           `json:"name"`
	Enabled               bool             `json:"enabled"`
	PostOnly              bool             `json:"postOnly"`
	PriceIncrement        fixedpoint.Value `json:"priceIncrement"`
	SizeIncrement         fixedpoint.Value `json:"sizeIncrement"`
	MinProvideSize        fixedpoint.Value `json:"minProvideSize"`
	Last                  fixedpoint.Value `json:"last"`
	Bid                   fixedpoint.Value `json:"bid"`
	Ask                   fixedpoint.Value `json:"ask"`
	Price                 fixedpoint.Value `json:"price"`
	Type                  string           `json:"type"`
	BaseCurrency          string           `json:"baseCurrency"`
	QuoteCurrency         string           `json:"quoteCurrency"`
	Underlying            string           `json:"underlying"`
	Restricted            bool             `json:"restricted"`
	HighLeverageFeeExempt bool             `json:"highLeverageFeeExempt"`
	Change1h              fixedpoint.Value `json:"change1h"`
	Change24h             fixedpoint.Value `json:"change24h"`
	ChangeBod             fixedpoint.Value `json:"changeBod"`
	QuoteVolume24h        fixedpoint.Value `json:"quoteVolume24h"`
	VolumeUsd24h          fixedpoint.Value `json:"volumeUsd24h"`
}

/*
{
  "success": true,
  "result": [
    {
      "close": 11055.25,
      "high": 11089.0,
      "low": 11043.5,
      "open": 11059.25,
      "startTime": "2019-06-24T17:15:00+00:00",
      "volume": 464193.95725
    }
  ]
}
*/
type HistoricalPricesResponse struct {
	Success bool     `json:"success"`
	Result  []Candle `json:"result"`
}

type Candle struct {
	Close     fixedpoint.Value `json:"close"`
	High      fixedpoint.Value `json:"high"`
	Low       fixedpoint.Value `json:"low"`
	Open      fixedpoint.Value `json:"open"`
	StartTime datetime         `json:"startTime"`
	Volume    fixedpoint.Value `json:"volume"`
}

type ordersHistoryResponse struct { // nolint:golint,deadcode
	Success     bool    `json:"success"`
	Result      []order `json:"result"`
	HasMoreData bool    `json:"hasMoreData"`
}

type ordersResponse struct { // nolint:golint,deadcode
	Success bool `json:"success"`

	Result []order `json:"result"`
}

type cancelOrderResponse struct { // nolint:golint,deadcode
	Success bool   `json:"success"`
	Result  string `json:"result"`
}

type order struct {
	CreatedAt  datetime         `json:"createdAt"`
	FilledSize fixedpoint.Value `json:"filledSize"`
	// Future field is not defined in the response format table but in the response example.
	Future        string           `json:"future"`
	ID            int64            `json:"id"`
	Market        string           `json:"market"`
	Price         fixedpoint.Value `json:"price"`
	AvgFillPrice  fixedpoint.Value `json:"avgFillPrice"`
	RemainingSize fixedpoint.Value `json:"remainingSize"`
	Side          string           `json:"side"`
	Size          fixedpoint.Value `json:"size"`
	Status        string           `json:"status"`
	Type          string           `json:"type"`
	ReduceOnly    bool             `json:"reduceOnly"`
	Ioc           bool             `json:"ioc"`
	PostOnly      bool             `json:"postOnly"`
	ClientId      string           `json:"clientId"`
	Liquidation   bool             `json:"liquidation"`
}

type orderResponse struct {
	Success bool `json:"success"`

	Result order `json:"result"`
}

/*
{
  "success": true,
  "result": [
    {
      "coin": "TUSD",
      "confirmations": 64,
      "confirmedTime": "2019-03-05T09:56:55.728933+00:00",
      "fee": 0,
      "id": 1,
      "sentTime": "2019-03-05T09:56:55.735929+00:00",
      "size": 99.0,
      "status": "confirmed",
      "time": "2019-03-05T09:56:55.728933+00:00",
      "txid": "0x8078356ae4b06a036d64747546c274af19581f1c78c510b60505798a7ffcaf1"
    }
  ]
}
*/
type depositHistoryResponse struct {
	Success bool             `json:"success"`
	Result  []depositHistory `json:"result"`
}

type depositHistory struct {
	ID            int64            `json:"id"`
	Coin          string           `json:"coin"`
	TxID          string           `json:"txid"`
	Address       address          `json:"address"`
	Confirmations int64            `json:"confirmations"`
	ConfirmedTime datetime         `json:"confirmedTime"`
	Fee           fixedpoint.Value `json:"fee"`
	SentTime      datetime         `json:"sentTime"`
	Size          fixedpoint.Value `json:"size"`
	Status        string           `json:"status"`
	Time          datetime         `json:"time"`
	Notes         string           `json:"notes"`
}

/**
{
	"address": "test123",
	"tag": null,
	"method": "ltc",
	"coin": null
}
*/
type address struct {
	Address string `json:"address"`
	Tag     string `json:"tag"`
	Method  string `json:"method"`
	Coin    string `json:"coin"`
}

type fillsResponse struct {
	Success bool   `json:"success"`
	Result  []fill `json:"result"`
}

/*
{
  "id": 123,
  "market": "TSLA/USD",
  "future": null,
  "baseCurrency": "TSLA",
  "quoteCurrency": "USD",
  "type": "order",
  "side": "sell",
  "price": 672.5,
  "size": 1.0,
  "orderId": 456,
  "time": "2021-02-23T09:29:08.534000+00:00",
  "tradeId": 789,
  "feeRate": -5e-6,
  "fee": -0.0033625,
  "feeCurrency": "USD",
  "liquidity": "maker"
}
*/
type fill struct {
	ID            int64            `json:"id"`
	Market        string           `json:"market"`
	Future        string           `json:"future"`
	BaseCurrency  string           `json:"baseCurrency"`
	QuoteCurrency string           `json:"quoteCurrency"`
	Type          string           `json:"type"`
	Side          types.SideType   `json:"side"`
	Price         fixedpoint.Value `json:"price"`
	Size          fixedpoint.Value `json:"size"`
	OrderId       uint64           `json:"orderId"`
	Time          datetime         `json:"time"`
	TradeId       uint64           `json:"tradeId"`
	FeeRate       fixedpoint.Value `json:"feeRate"`
	Fee           fixedpoint.Value `json:"fee"`
	FeeCurrency   string           `json:"feeCurrency"`
	Liquidity     string           `json:"liquidity"`
}

type transferResponse struct {
	Success bool     `json:"success"`
	Result  transfer `json:"result"`
}

type transfer struct {
	Id     uint             `json:"id"`
	Coin   string           `json:"coin"`
	Size   fixedpoint.Value `json:"size"`
	Time   string           `json:"time"`
	Notes  string           `json:"notes"`
	Status string           `json:"status"`
}

func (t *transfer) String() string {
	return fmt.Sprintf("%+v", *t)
}
