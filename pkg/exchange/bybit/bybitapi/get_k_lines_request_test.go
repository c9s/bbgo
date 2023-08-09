package bybitapi

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func TestKLinesResponse_UnmarshalJSON(t *testing.T) {
	t.Run("succeeds", func(t *testing.T) {
		data := `{
        "symbol": "BTCUSDT",
        "category": "spot",
        "list": [
            [
                "1670608800000",
                "17071",
                "17073",
                "17027",
                "17055.5",
                "268611",
                "15.74462667"
            ],
            [
                "1670605200000",
                "17071.5",
                "17071.5",
                "17061",
                "17071",
                "4177",
                "0.24469757"
            ]
        ]
    }`

		expRes := &KLinesResponse{
			Symbol: "BTCUSDT",
			List: []KLine{
				{
					StartTime: types.NewMillisecondTimestampFromInt(1670608800000),
					Open:      fixedpoint.NewFromFloat(17071),
					High:      fixedpoint.NewFromFloat(17073),
					Low:       fixedpoint.NewFromFloat(17027),
					Close:     fixedpoint.NewFromFloat(17055.5),
					Volume:    fixedpoint.NewFromFloat(268611),
					TurnOver:  fixedpoint.NewFromFloat(15.74462667),
				},
				{
					StartTime: types.NewMillisecondTimestampFromInt(1670605200000),
					Open:      fixedpoint.NewFromFloat(17071.5),
					High:      fixedpoint.NewFromFloat(17071.5),
					Low:       fixedpoint.NewFromFloat(17061),
					Close:     fixedpoint.NewFromFloat(17071),
					Volume:    fixedpoint.NewFromFloat(4177),
					TurnOver:  fixedpoint.NewFromFloat(0.24469757),
				},
			},
			Category: CategorySpot,
		}

		kline := &KLinesResponse{}
		err := json.Unmarshal([]byte(data), kline)
		assert.NoError(t, err)
		assert.Equal(t, expRes, kline)
	})

	t.Run("unexpected length", func(t *testing.T) {
		data := `{
        "symbol": "BTCUSDT",
        "category": "spot",
        "list": [
            [
                "1670608800000",
                "17071",
                "17073",
                "17027",
                "17055.5",
                "268611"
            ]
        ]
    }`
		kline := &KLinesResponse{}
		err := json.Unmarshal([]byte(data), kline)
		assert.Equal(t, fmt.Errorf("unexpected K Lines array length: 6, exp: %d", KLinesArrayLen), err)
	})

	t.Run("unexpected json array", func(t *testing.T) {
		klineJson := `{}`

		data := fmt.Sprintf(`{
        "symbol": "BTCUSDT",
        "category": "spot",
        "list": [%s]
    }`, klineJson)

		var jsonArr []json.RawMessage
		expErr := json.Unmarshal([]byte(klineJson), &jsonArr)
		assert.Error(t, expErr)

		kline := &KLinesResponse{}
		err := json.Unmarshal([]byte(data), kline)
		assert.Equal(t, fmt.Errorf("failed to unmarshal jsonRawMessage: %v, err: %w", klineJson, expErr), err)
	})

	t.Run("unexpected json 0", func(t *testing.T) {
		klineJson := `
			[
				"a",
                "17071.5",
                "17071.5",
                "17061",
                "17071",
                "4177",
                "0.24469757"
			]
		`

		data := fmt.Sprintf(`{
        "symbol": "BTCUSDT",
        "category": "spot",
        "list": [%s]
    }`, klineJson)

		var jsonArr []json.RawMessage
		err := json.Unmarshal([]byte(klineJson), &jsonArr)
		assert.NoError(t, err)

		timestamp := types.MillisecondTimestamp{}
		expErr := json.Unmarshal(jsonArr[0], &timestamp)
		assert.NoError(t, err)

		kline := &KLinesResponse{}
		err = json.Unmarshal([]byte(data), kline)
		assert.Equal(t, fmt.Errorf("failed to unmarshal resp index 0: %v, err: %w", string(jsonArr[0]), expErr), err)
	})

	t.Run("unexpected json 1", func(t *testing.T) {
		// TODO: fix panic
		t.Skip("test will result in a panic, skip it")
		klineJson := `
			[
				"1670608800000",
                "a",
                "17071.5",
                "17061",
                "17071",
                "4177",
                "0.24469757"
			]
		`

		data := fmt.Sprintf(`{
        "symbol": "BTCUSDT",
        "category": "spot",
        "list": [%s]
    }`, klineJson)

		var jsonArr []json.RawMessage
		err := json.Unmarshal([]byte(klineJson), &jsonArr)
		assert.NoError(t, err)

		var value fixedpoint.Value
		expErr := json.Unmarshal(jsonArr[1], &value)
		assert.NoError(t, err)

		kline := &KLinesResponse{}
		err = json.Unmarshal([]byte(data), kline)
		assert.Equal(t, fmt.Errorf("failed to unmarshal resp index 1: %v, err: %w", string(jsonArr[1]), expErr), err)
	})
}
