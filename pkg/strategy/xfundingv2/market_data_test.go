package xfundingv2

import (
	"context"
	"os"
	"testing"

	"github.com/c9s/bbgo/pkg/datasource/coinmarketcap"
	"github.com/stretchr/testify/assert"
)

func Test_queryTopCapAssets(t *testing.T) {
	apiKey := os.Getenv("COINMARKETCAP_API_KEY")
	if apiKey == "" {
		t.Skip("COINMARKETCAP_API_KEY not set, skipping test")
	}
	strategy := &Strategy{
		MarketSelectionConfig: &MarketSelectionConfig{
			TopNCap: 10,
		},
		coinmarketcapClient: coinmarketcap.New(apiKey),
	}
	ctx := context.Background()
	topCapMarkets, err := strategy.queryTopCapAssets(ctx)

	assert.NoError(t, err)
	assert.NotEmpty(t, topCapMarkets)
}

func Test_queryPortfolioModeCollateralRates(t *testing.T) {
	ctx := context.Background()
	symbols := []string{"BTC", "ETH", "BNB"}
	collateralRates, err := queryPortfolioModeCollateralRates(ctx, symbols)

	assert.NoError(t, err)
	assert.NotEmpty(t, collateralRates)
}
