package bybitapi

import (
	"context"
	"encoding/json"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/requestgen"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type Tickers struct {
	Category Category `json:"category"`
	List     []Ticker `json:"list"`

	// ClosedTime is current timestamp (ms). This value is obtained from outside APIResponse.
	ClosedTime types.MillisecondTimestamp
}

type Ticker struct {
	Symbol        string           `json:"symbol"`
	Bid1Price     fixedpoint.Value `json:"bid1Price"`
	Bid1Size      fixedpoint.Value `json:"bid1Size"`
	Ask1Price     fixedpoint.Value `json:"ask1Price"`
	Ask1Size      fixedpoint.Value `json:"ask1Size"`
	LastPrice     fixedpoint.Value `json:"lastPrice"`
	PrevPrice24H  fixedpoint.Value `json:"prevPrice24h"`
	Price24HPcnt  fixedpoint.Value `json:"price24hPcnt"`
	HighPrice24H  fixedpoint.Value `json:"highPrice24h"`
	LowPrice24H   fixedpoint.Value `json:"lowPrice24h"`
	Turnover24H   fixedpoint.Value `json:"turnover24h"`
	Volume24H     fixedpoint.Value `json:"volume24h"`
	UsdIndexPrice fixedpoint.Value `json:"usdIndexPrice"`
}

// GetTickersRequest without **-responseDataType .InstrumentsInfo** in generation command, because the caller
// needs the APIResponse.Time. We implemented the DoWithResponseTime to handle this.
//
//go:generate GetRequest -url "/v5/market/tickers" -type GetTickersRequest
type GetTickersRequest struct {
	client requestgen.APIClient

	category Category `param:"category,query" validValues:"spot"`
	symbol   *string  `param:"symbol,query"`
}

func (c *RestClient) NewGetTickersRequest() *GetTickersRequest {
	return &GetTickersRequest{
		client:   c,
		category: CategorySpot,
	}
}

func (g *GetTickersRequest) DoWithResponseTime(ctx context.Context) (*Tickers, error) {
	resp, err := g.Do(ctx)
	if err != nil {
		return nil, err
	}

	var data Tickers
	if err := json.Unmarshal(resp.Result, &data); err != nil {
		return nil, err
	}

	// Our types.Ticker requires the closed time, but this API does not provide it. This API returns the Tickers of the
	// past 24 hours, so in terms of closed time, it is the current time, so fill it in Tickers.ClosedTime.
	data.ClosedTime = resp.Time
	return &data, nil
}
