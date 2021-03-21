package ftx

import "time"

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
type accountResponse struct {
	Success bool    `json:"success"`
	Result  account `json:"result"`
}

type account struct {
	MakerFee float64 `json:"makerFee"`
	TakerFee float64 `json:"takerFee"`
}

type positionsResponse struct {
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
	Cost                         float64 `json:"cost"`
	EntryPrice                   float64 `json:"entryPrice"`
	EstimatedLiquidationPrice    float64 `json:"estimatedLiquidationPrice"`
	Future                       string  `json:"future"`
	InitialMarginRequirement     float64 `json:"initialMarginRequirement"`
	LongOrderSize                float64 `json:"longOrderSize"`
	MaintenanceMarginRequirement float64 `json:"maintenanceMarginRequirement"`
	NetSize                      float64 `json:"netSize"`
	OpenSize                     float64 `json:"openSize"`
	RealizedPnl                  float64 `json:"realizedPnl"`
	ShortOrderSize               float64 `json:"shortOrderSize"`
	Side                         string  `json:"Side"`
	Size                         float64 `json:"size"`
	UnrealizedPnl                float64 `json:"unrealizedPnl"`
	CollateralUsed               float64 `json:"collateralUsed"`
}

type balances struct {
	Success bool `json:"success"`

	Result []struct {
		Coin  string  `json:"coin"`
		Free  float64 `json:"free"`
		Total float64 `json:"total"`
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
type marketsResponse struct {
	Success bool     `json:"success"`
	Result  []market `json:"result"`
}

type market struct {
	Name                  string  `json:"name"`
	Enabled               bool    `json:"enabled"`
	PostOnly              bool    `json:"postOnly"`
	PriceIncrement        float64 `json:"priceIncrement"`
	SizeIncrement         float64 `json:"sizeIncrement"`
	MinProvideSize        float64 `json:"minProvideSize"`
	Last                  float64 `json:"last"`
	Bid                   float64 `json:"bid"`
	Ask                   float64 `json:"ask"`
	Price                 float64 `json:"price"`
	Type                  string  `json:"type"`
	BaseCurrency          string  `json:"baseCurrency"`
	QuoteCurrency         string  `json:"quoteCurrency"`
	Underlying            string  `json:"underlying"`
	Restricted            bool    `json:"restricted"`
	HighLeverageFeeExempt bool    `json:"highLeverageFeeExempt"`
	Change1h              float64 `json:"change1h"`
	Change24h             float64 `json:"change24h"`
	ChangeBod             float64 `json:"changeBod"`
	QuoteVolume24h        float64 `json:"quoteVolume24h"`
	VolumeUsd24h          float64 `json:"volumeUsd24h"`
}

type ordersHistoryResponse struct {
	Success     bool    `json:"success"`
	Result      []order `json:"result"`
	HasMoreData bool    `json:"hasMoreData"`
}

type ordersResponse struct {
	Success bool `json:"success"`

	Result []order `json:"result"`
}

type cancelOrderResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
}

type order struct {
	CreatedAt  time.Time `json:"createdAt"`
	FilledSize float64   `json:"filledSize"`
	// Future field is not defined in the response format table but in the response example.
	Future        string  `json:"future"`
	ID            int64   `json:"id"`
	Market        string  `json:"market"`
	Price         float64 `json:"price"`
	AvgFillPrice  float64 `json:"avgFillPrice"`
	RemainingSize float64 `json:"remainingSize"`
	Side          string  `json:"side"`
	Size          float64 `json:"size"`
	Status        string  `json:"status"`
	Type          string  `json:"type"`
	ReduceOnly    bool    `json:"reduceOnly"`
	Ioc           bool    `json:"ioc"`
	PostOnly      bool    `json:"postOnly"`
	ClientId      string  `json:"clientId"`
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
	ID            int64     `json:"id"`
	Coin          string    `json:"coin"`
	TxID          string    `json:"txid"`
	Address       address   `json:"address"`
	Confirmations int64     `json:"confirmations"`
	ConfirmedTime time.Time `json:"confirmedTime"`
	Fee           float64   `json:"fee"`
	SentTime      time.Time `json:"sentTime"`
	Size          float64   `json:"size"`
	Status        string    `json:"status"`
	Time          time.Time `json:"time"`
	Notes         string    `json:"notes"`
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
