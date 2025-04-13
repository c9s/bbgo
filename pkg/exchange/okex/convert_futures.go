package okex

import (
	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalFuturesAccountInfo(account *okexapi.Account, positions []okexapi.Position) *types.FuturesAccountInfo {
	return &types.FuturesAccountInfo{
		Assets:                      toGlobalFuturesUserAssets(account.Details),
		Positions:                   toGlobalFuturesPositions(positions),
		TotalInitialMargin:          account.TotalInitialMargin,
		TotalMaintMargin:            account.TotalMaintMargin,
		TotalMarginBalance:          account.TotalEquityInUSD,
		TotalUnrealizedProfit:       account.UnrealizedPnl,
		TotalWalletBalance:          account.TotalEquityInUSD,
		TotalOpenOrderInitialMargin: account.TotalOpenOrderInitialMargin,
		UpdateTime:                  account.UpdateTime.Time().Unix(),
	}
}

func toGlobalFuturesUserAssets(assets []okexapi.BalanceDetail) (retAssets types.FuturesAssetMap) {
	retFuturesAssets := make(types.FuturesAssetMap)
	for _, detail := range assets {
		retFuturesAssets[detail.Currency] = types.FuturesUserAsset{
			Asset:                  detail.Currency,
			InitialMargin:          fixedpoint.MustNewFromString(detail.Imr),
			MaintMargin:            fixedpoint.MustNewFromString(detail.Mmr),
			MarginBalance:          detail.EquityInUSD,
			MaxWithdrawAmount:      detail.Available,
			OpenOrderInitialMargin: fixedpoint.Zero,
			PositionInitialMargin:  fixedpoint.MustNewFromString(detail.Imr),
			UnrealizedProfit:       detail.UnrealizedProfitAndLoss,
			WalletBalance:          detail.EquityInUSD,
		}
	}
	return retFuturesAssets
}

func toGlobalFuturesPositions(futuresPositions []okexapi.Position) types.FuturesPositionMap {
	retFuturesPositions := make(types.FuturesPositionMap)
	for _, futuresPosition := range futuresPositions {
		isolated := futuresPosition.MgnMode == okexapi.MarginModeIsolated
		symbol := toGlobalSymbol(futuresPosition.InstId)
		retFuturesPositions[symbol] = types.FuturesPosition{
			Isolated:    isolated,
			AverageCost: futuresPosition.AvgPx,
			Base:        futuresPosition.Pos,
			Quote:       futuresPosition.Pos.Mul(futuresPosition.MarkPx),
			PositionRisk: &types.PositionRisk{
				Leverage: futuresPosition.Lever,
			},
			Symbol:     symbol,
			UpdateTime: futuresPosition.UpdatedTime.Time().Unix(),
		}
	}

	return retFuturesPositions
}
