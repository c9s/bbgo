package bybitapi

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/c9s/requestgen"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

//go:generate -command GetRequest requestgen -method GET -responseType .APIResponse -responseDataField Result
//go:generate -command PostRequest requestgen -method POST -responseType .APIResponse -responseDataField Result

type IntervalSign string

const (
	IntervalSignDay   IntervalSign = "D"
	IntervalSignWeek  IntervalSign = "W"
	IntervalSignMonth IntervalSign = "M"
)

type KLinesResponse struct {
	Symbol string `json:"symbol"`
	// An string array of individual candle
	// Sort in reverse by startTime
	List     []KLine  `json:"list"`
	Category Category `json:"category"`
}

type KLine struct {
	// list[0]: startTime, Start time of the candle (ms)
	StartTime types.MillisecondTimestamp
	// list[1]: openPrice
	Open fixedpoint.Value
	// list[2]: highPrice
	High fixedpoint.Value
	// list[3]: lowPrice
	Low fixedpoint.Value
	// list[4]: closePrice
	Close fixedpoint.Value
	// list[5]: volume, Trade volume. Unit of contract: pieces of contract. Unit of spot: quantity of coins
	Volume fixedpoint.Value
	// list[6]: turnover, Turnover. Unit of figure: quantity of quota coin
	TurnOver fixedpoint.Value
}

const KLinesArrayLen = 7

func (k *KLine) UnmarshalJSON(data []byte) error {
	var jsonArr []json.RawMessage
	err := json.Unmarshal(data, &jsonArr)
	if err != nil {
		return fmt.Errorf("failed to unmarshal jsonRawMessage: %v, err: %w", string(data), err)
	}
	if len(jsonArr) != KLinesArrayLen {
		return fmt.Errorf("unexpected K Lines array length: %d, exp: %d", len(jsonArr), KLinesArrayLen)
	}

	err = json.Unmarshal(jsonArr[0], &k.StartTime)
	if err != nil {
		return fmt.Errorf("failed to unmarshal resp index 0: %v, err: %w", string(jsonArr[0]), err)
	}

	values := make([]fixedpoint.Value, len(jsonArr)-1)
	for i, jsonRaw := range jsonArr[1:] {
		err = json.Unmarshal(jsonRaw, &values[i])
		if err != nil {
			return fmt.Errorf("failed to unmarshal resp index %d: %v, err: %w", i+1, string(jsonRaw), err)
		}
	}
	k.Open = values[0]
	k.High = values[1]
	k.Low = values[2]
	k.Close = values[3]
	k.Volume = values[4]
	k.TurnOver = values[5]

	return nil
}

//go:generate GetRequest -url "/v5/market/kline" -type GetKLinesRequest -responseDataType .KLinesResponse
type GetKLinesRequest struct {
	client requestgen.APIClient

	category Category `param:"category,query" validValues:"spot"`
	symbol   string   `param:"symbol,query"`
	// Kline interval.
	// - 1,3,5,15,30,60,120,240,360,720: minute
	// - D: day
	// - M: month
	// - W: week
	interval  string     `param:"interval,query" validValues:"1,3,5,15,30,60,120,240,360,720,D,W,M"`
	startTime *time.Time `param:"start,query,milliseconds"`
	endTime   *time.Time `param:"end,query,milliseconds"`
	// Limit for data size per page. [1, 1000]. Default: 200
	limit *uint64 `param:"limit,query"`
}

func (c *RestClient) NewGetKLinesRequest() *GetKLinesRequest {
	return &GetKLinesRequest{
		client:   c,
		category: CategorySpot,
	}
}
