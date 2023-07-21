package bybitapi

import (
	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type InstrumentsInfo struct {
	Category Category `json:"category"`
	List     []struct {
		Symbol        string `json:"symbol"`
		BaseCoin      string `json:"baseCoin"`
		QuoteCoin     string `json:"quoteCoin"`
		Innovation    string `json:"innovation"`
		Status        Status `json:"status"`
		MarginTrading string `json:"marginTrading"`
		LotSizeFilter struct {
			BasePrecision  fixedpoint.Value `json:"basePrecision"`
			QuotePrecision fixedpoint.Value `json:"quotePrecision"`
			MinOrderQty    fixedpoint.Value `json:"minOrderQty"`
			MaxOrderQty    fixedpoint.Value `json:"maxOrderQty"`
			MinOrderAmt    fixedpoint.Value `json:"minOrderAmt"`
			MaxOrderAmt    fixedpoint.Value `json:"maxOrderAmt"`
		} `json:"lotSizeFilter"`

		PriceFilter struct {
			TickSize fixedpoint.Value `json:"tickSize"`
		} `json:"priceFilter"`
	} `json:"list"`
}

//go:generate GetRequest -url "/v5/market/instruments-info" -type GetInstrumentsInfoRequest -responseDataType .InstrumentsInfo
type GetInstrumentsInfoRequest struct {
	client requestgen.APIClient

	category Category `param:"category,query" validValues:"spot"`
	symbol   *string  `param:"symbol,query"`
	limit    *uint64  `param:"limit,query"`
	cursor   *string  `param:"cursor,query"`
}

func (c *RestClient) NewGetInstrumentsInfoRequest() *GetInstrumentsInfoRequest {
	return &GetInstrumentsInfoRequest{
		client:   c,
		category: CategorySpot,
	}
}
