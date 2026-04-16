package xfundingv2

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const binancePublicFuturesAPI = "https://www.binance.com/bapi/margin/v1/public/margin/portfolio/collateral-rate"

func (s *Strategy) queryTopCapAssets(ctx context.Context) (map[string]float64, error) {
	if s.coinmarketcapClient == nil {
		return nil, errors.New("query top cap markets disabled")
	}
	topCapAssets, err := s.coinmarketcapClient.QueryMarketCapInUSD(ctx, s.MarketSelectionConfig.TopNCap)
	if err != nil {
		return nil, err
	}
	return topCapAssets, nil
}

type BinanceFuturesResponse struct {
	Data []struct {
		Asset          string           `json:"asset"`
		CollateralRate fixedpoint.Value `json:"collateralRate"`
		UpdateTime     int64            `json:"updateTime"`
		Virtual        bool             `json:"virtual"`
		DiscountFactor fixedpoint.Value `json:"discountFactor"`
	} `json:"data"`
}

func queryPortfolioModeCollateralRates(ctx context.Context, assets []string) (map[string]fixedpoint.Value, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		resp, err := http.Get(binancePublicFuturesAPI)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var apiResp BinanceFuturesResponse
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			return nil, err
		}

		symbolsMap := make(map[string]struct{}, len(assets))
		for _, symbol := range assets {
			symbolsMap[symbol] = struct{}{}
		}

		collateralRates := make(map[string]fixedpoint.Value)
		for _, data := range apiResp.Data {
			if _, ok := symbolsMap[data.Asset]; !ok {
				continue
			}
			collateralRates[data.Asset] = data.CollateralRate
		}
		return collateralRates, nil
	}
}
