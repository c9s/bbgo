package max

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicService(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()
	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	t.Run("v2/timestamp", func(t *testing.T) {
		req := client.NewGetTimestampRequest()
		serverTimestamp, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotZero(t, serverTimestamp)
	})

	t.Run("v2/tickers", func(t *testing.T) {
		req := client.NewGetTickersRequest()
		tickers, err := req.Do(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, tickers)
		assert.NotEmpty(t, tickers)
		assert.NotEmpty(t, tickers["btcusdt"])
	})
}

func TestRewardService_GetRewardsRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	req := client.RewardService.NewGetRewardsRequest()
	rewards, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, rewards)
	assert.NotEmpty(t, rewards)

	t.Logf("rewards: %+v", rewards)
}

func TestRewardService_GetRewardsOfTypeRequest(t *testing.T) {
	key, secret, ok := integrationTestConfigured(t, "MAX")
	if !ok {
		t.SkipNow()
	}

	ctx := context.Background()

	client := NewRestClient(ProductionAPIURL)
	client.Auth(key, secret)

	req := client.RewardService.NewGetRewardsOfTypeRequest(RewardCommission)
	rewards, err := req.Do(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, rewards)
	assert.NotEmpty(t, rewards)

	t.Logf("rewards: %+v", rewards)

	for _, reward := range rewards {
		assert.Equal(t, RewardCommission, reward.Type)
	}
}
