package max

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewardService_GetRewardsRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
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
		req := client.RewardService.NewGetRewardsOfTypeRequest(RewardCommission)
		rewards, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, rewards)
		assert.NotEmpty(t, rewards)

		t.Logf("rewards: %+v", rewards)

		for _, reward := range rewards {
			assert.Equal(t, RewardCommission, reward.Type)
		}
	})
}
