package maxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestPublicService(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	client := NewRestClientDefault()
	_ = key
	_ = secret

	t.Run("v2/k", func(t *testing.T) {
		req := client.NewGetKLinesRequest()
		data, err := req.Market("btcusdt").Period(int(60)).Limit(100).Do(ctx)
		assert.NoError(t, err)
		if assert.NotEmpty(t, data) {
			assert.NotEmpty(t, data[0])
			assert.Len(t, data[0], 6)
		}
	})
}
