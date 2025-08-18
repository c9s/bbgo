package okexapi

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Data
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Data

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type KLine struct {
	StartTime    types.MillisecondTimestamp
	OpenPrice    fixedpoint.Value
	HighestPrice fixedpoint.Value
	LowestPrice  fixedpoint.Value
	ClosePrice   fixedpoint.Value
	// Volume trading volume, with a unit of contract.

	// If it is a derivatives contract, the value is the number of contracts.
	// If it is SPOT/MARGIN, the value is the quantity in base currency.
	Volume fixedpoint.Value
	// VolumeInCurrency trading volume, with a unit of currency.
	// If it is a derivatives contract, the value is the number of base currency.
	// If it is SPOT/MARGIN, the value is the quantity in quote currency.
	VolumeInCurrency fixedpoint.Value
	// VolumeInCurrencyQuote Trading volume, the value is the quantity in quote currency
	// e.g. The unit is USDT for BTC-USDT and BTC-USDT-SWAP;
	// The unit is USD for BTC-USD-SWAP
	// ** REMARK: To prevent overflow, we need to avoid unmarshaling it.  **
	//VolumeInCurrencyQuote fixedpoint.Value
	// The state of candlesticks.
	// 0 represents that it is uncompleted, 1 represents that it is completed.
	Confirm fixedpoint.Value
}

type KLineSlice []KLine

func (m *KLineSlice) UnmarshalJSON(b []byte) error {
	if m == nil {
		return errors.New("nil pointer of kline slice")
	}
	s, err := parseKLineSliceJSON(b)
	if err != nil {
		return err
	}

	*m = s
	return nil
}

// parseKLineSliceJSON tries to parse a 2 dimensional string array into a KLineSlice
//
//		[
//	   [
//	     "1597026383085",
//	     "8533.02",
//	     "8553.74",
//	     "8527.17",
//	     "8548.26",
//	     "45247",
//	     "529.5858061",
//	     "5529.5858061",
//	     "0"
//	   ]
//	 ]
func parseKLineSliceJSON(in []byte) (slice KLineSlice, err error) {
	var rawKLines [][]json.RawMessage

	err = json.Unmarshal(in, &rawKLines)
	if err != nil {
		return slice, err
	}

	for _, raw := range rawKLines {
		if len(raw) != 9 {
			return nil, fmt.Errorf("unexpected kline length: %d, data: %q", len(raw), raw)
		}
		var kline KLine
		if err = json.Unmarshal(raw[0], &kline.StartTime); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into timestamp: %q", raw[0])
		}
		if err = json.Unmarshal(raw[1], &kline.OpenPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into open price: %q", raw[1])
		}
		if err = json.Unmarshal(raw[2], &kline.HighestPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into highest price: %q", raw[2])
		}
		if err = json.Unmarshal(raw[3], &kline.LowestPrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into lowest price: %q", raw[3])
		}
		if err = json.Unmarshal(raw[4], &kline.ClosePrice); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into close price: %q", raw[4])
		}
		if err = json.Unmarshal(raw[5], &kline.Volume); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into volume: %q", raw[5])
		}
		if err = json.Unmarshal(raw[6], &kline.VolumeInCurrency); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into volume currency: %q", raw[6])
		}
		//if err = json.Unmarshal(raw[7], &kline.VolumeInCurrencyQuote); err != nil {
		//	return nil, fmt.Errorf("failed to unmarshal into trading currency quote: %q", raw[7])
		//}
		if err = json.Unmarshal(raw[8], &kline.Confirm); err != nil {
			return nil, fmt.Errorf("failed to unmarshal into confirm: %q", raw[8])
		}

		slice = append(slice, kline)
	}

	return slice, nil
}

//go:generate GetRequest -url "/api/v5/market/candles" -type GetCandlesRequest -responseDataType KLineSlice
type GetCandlesRequest struct {
	client requestgen.APIClient

	instrumentID string `param:"instId,query"`

	limit *int `param:"limit,query"`

	bar *string `param:"bar,query"`

	after *time.Time `param:"after,query,milliseconds"`

	before *time.Time `param:"before,query,milliseconds"`
}

func (c *RestClient) NewGetCandlesRequest() *GetCandlesRequest {
	return &GetCandlesRequest{client: c}
}
