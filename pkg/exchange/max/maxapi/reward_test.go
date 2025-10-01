package maxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/testutil"
)

func TestRewards(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClientDefault()
	client.Auth(key, secret)

	t.Run("v2/rewards", func(t *testing.T) {
		req := client.RewardService.NewGetRewardsRequest()
		rewards, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, rewards)
		assert.NotEmpty(t, rewards)
		t.Logf("rewards: %+v", rewards)
	})

	t.Run("v2/rewards with type", func(t *testing.T) {
		req := client.RewardService.NewGetRewardsOfTypeRequest(RewardHolding)
		rewards, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, rewards)
		assert.NotEmpty(t, rewards)

		t.Logf("rewards: %+v", rewards)

		for _, reward := range rewards {
			assert.Equal(t, RewardHolding, reward.Type)
		}
	})
}
