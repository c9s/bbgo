package binance

import (
	"context"
	"strconv"
	"time"

	"github.com/c9s/bbgo/pkg/exchange/binance/binanceapi"
	"github.com/c9s/bbgo/pkg/types"
)

func (e *Exchange) QueryRewards(ctx context.Context, startTime time.Time) ([]types.Reward, error) {
	req := e.client2.NewGetSpotRebateHistoryRequest()
	req.StartTime(startTime)
	history, err := req.Do(ctx)
	if err != nil {
		return nil, err
	}

	var rewards []types.Reward

	for _, entry := range history {
		t := types.RewardCommission
		switch entry.Type {
		case binanceapi.RebateTypeReferralKickback:
			t = types.RewardReferralKickback
		case binanceapi.RebateTypeCommission:
			// use the default type
		}

		rewards = append(rewards, types.Reward{
			UUID:      strconv.FormatInt(entry.UpdateTime.Time().UnixMilli(), 10),
			Exchange:  types.ExchangeBinance,
			Type:      t,
			Currency:  entry.Asset,
			Quantity:  entry.Amount,
			State:     "done",
			Note:      "",
			Spent:     false,
			CreatedAt: types.Time(entry.UpdateTime),
		})
	}

	return rewards, nil
}
