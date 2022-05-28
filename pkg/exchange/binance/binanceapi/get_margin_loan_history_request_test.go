package binanceapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_GetMarginLoanHistoryRequest(t *testing.T) {
	client := getTestClientOrSkip(t)
	ctx := context.Background()

	err := client.SetTimeOffsetFromServer(ctx)
	assert.NoError(t, err)

	req := client.NewGetMarginLoanHistoryRequest()
	req.Asset("USDT")
	req.IsolatedSymbol("DOTUSDT")
	req.StartTime(time.Date(2022, time.February, 1, 0, 0, 0, 0, time.UTC))
	req.EndTime(time.Date(2022, time.March, 1, 0, 0, 0, 0, time.UTC))
	req.Size(100)

	records, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, records)
	t.Logf("loans: %+v", records)
}
