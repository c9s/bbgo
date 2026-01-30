package maxapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	v3 "github.com/c9s/bbgo/pkg/exchange/max/maxapi/v3"
	"github.com/c9s/bbgo/pkg/testutil"
)

func TestRewards(t *testing.T) {
	key, secret, ok := testutil.IntegrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := v3.NewClient()
	client.Auth(key, secret)

	t.Run("v2/rewards", func(t *testing.T) {
		req := client.RewardService.NewGetRewardsRequest()
		rewards, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, rewards)
		assert.NotEmpty(t, rewards)
		t.Logf("rewards: %+v", rewards)
	})
}
