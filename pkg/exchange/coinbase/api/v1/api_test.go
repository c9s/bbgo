package coinbase

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTransferTime(t *testing.T) {

	t.Run(
		"ZeroTimeZone", func(t *testing.T) {
			oriTime, err := time.Parse(time.RFC3339Nano, "2025-06-20T06:11:10.123456Z")
			assert.NoError(t, err)
			tt := TransferTime(oriTime)

			data, err := json.Marshal(tt)
			jsonStr := string(data)
			assert.NoError(t, err)
			assert.Equal(t, `"2025-06-20 06:11:10.123456Z"`, jsonStr)

			var tt2 TransferTime
			err = json.Unmarshal(data, &tt2)
			assert.NoError(t, err)
			assert.Equal(t, tt.Time(), tt2.Time())
		},
	)

	t.Run(
		"WithTimeZone", func(t *testing.T) {
			oriTime, err := time.Parse(time.RFC3339Nano, "2025-06-20T06:11:10.123456+08:00")
			assert.NoError(t, err)
			tt := TransferTime(oriTime)

			data, err := json.Marshal(tt)
			jsonStr := string(data)
			assert.NoError(t, err)
			assert.Equal(t, `"2025-06-20 06:11:10.123456+08"`, jsonStr)

			var tt2 TransferTime
			err = json.Unmarshal(data, &tt2)
			assert.NoError(t, err)
			assert.Equal(t, tt.Time(), tt2.Time())
		},
	)

}
