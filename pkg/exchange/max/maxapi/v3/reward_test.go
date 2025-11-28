package v3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/exchange/max/maxapi"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestReward(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	restClient := maxapi.NewRestClientDefault()
	restClient.Auth(key, secret)
	assert.NotNil(t, restClient)

	client := NewClient(restClient)
	req := client.NewGetRewardsRequest()
	rewards, err := req.Do(ctx)
	assert.NoError(t, err)

	t.Logf("rewards: %+v", rewards)
	for _, reward := range rewards {
		assert.True(t, !reward.Amount.IsZero())
		assert.NotEmpty(t, reward.Type)
		assert.NotEmpty(t, reward.Currency)
		assert.NotEmpty(t, reward.UUID)
	}
}
