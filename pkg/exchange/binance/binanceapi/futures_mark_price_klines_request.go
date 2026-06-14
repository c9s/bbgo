package binanceapi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

// FuturesKLine represents a single mark price / index price kline candlestick.
//
// The futures market data endpoints return klines as a JSON array of values, e.g.
//
//	[
//	  1591256400000,  // Open time
//	  "9647.25",      // Open
//	  "9650.0",       // High
//	  "9647.25",      // Low
//	  "9648.75",      // Close (or latest price)
//	  "0",            // Ignore
//	  1591256459999,  // Close time
//	  "0",            // Ignore
//	  60,             // Number of basic data
//	  "0",            // Ignore
//	  "0",            // Ignore
//	  "0"             // Ignore
//	]
type FuturesKLine struct {
	OpenTime  types.MillisecondTimestamp
	Open      fixedpoint.Value
	High      fixedpoint.Value
	Low       fixedpoint.Value
	Close     fixedpoint.Value
	CloseTime types.MillisecondTimestamp
}

func (k *FuturesKLine) UnmarshalJSON(data []byte) error {
	var values []json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}

	if len(values) < 7 {
		return fmt.Errorf("unexpected futures kline length %d: %s", len(values), string(data))
	}

	if err := json.Unmarshal(values[0], &k.OpenTime); err != nil {
		return err
	}
	if err := json.Unmarshal(values[1], &k.Open); err != nil {
		return err
	}
	if err := json.Unmarshal(values[2], &k.High); err != nil {
		return err
	}
	if err := json.Unmarshal(values[3], &k.Low); err != nil {
		return err
	}
	if err := json.Unmarshal(values[4], &k.Close); err != nil {
		return err
	}
	if err := json.Unmarshal(values[6], &k.CloseTime); err != nil {
		return err
	}

	return nil
}

//go:generate requestgen -method GET -url "/fapi/v1/markPriceKlines" -type FuturesMarkPriceKlinesRequest -responseType []FuturesKLine
type FuturesMarkPriceKlinesRequest struct {
	client requestgen.APIClient

	symbol   string         `param:"symbol,required"`
	interval types.Interval `param:"interval,required"`

	limit     *uint64    `param:"limit"`
	startTime *time.Time `param:"startTime,milliseconds"`
	endTime   *time.Time `param:"endTime,milliseconds"`
}

func (c *FuturesRestClient) NewFuturesMarkPriceKlinesRequest() *FuturesMarkPriceKlinesRequest {
	return &FuturesMarkPriceKlinesRequest{client: c}
}
