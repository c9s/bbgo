package okex

import (
	"github.com/c9s/bbgo/pkg/exchange/okex/okexapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

func toGlobalFuturesAccountInfo(account *okexapi.Account, positions []okexapi.Position) *types.FuturesAccount {
	return &types.FuturesAccount{
		Assets:                      toGlobalFuturesUserAssets(account.Details),
		Positions:                   toGlobalFuturesPositions(positions),
		TotalInitialMargin:          account.TotalInitialMargin,
		TotalMaintMargin:            account.TotalMaintMargin,
		TotalMarginBalance:          account.TotalEquityInUSD,
		TotalUnrealizedProfit:       account.UnrealizedPnl,
		TotalWalletBalance:          account.TotalEquityInUSD,
		TotalOpenOrderInitialMargin: account.TotalOpenOrderInitialMargin,
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
		positionSide := toGlobalPositionSide(okexapi.PosSide(futuresPosition.PosSide))

		posKey := types.NewPositionKey(symbol, positionSide)
		retFuturesPositions[posKey] = types.FuturesPosition{
			Isolated:     isolated,
			AverageCost:  futuresPosition.AvgPx,
			Base:         futuresPosition.Pos,
			Quote:        futuresPosition.Pos.Mul(futuresPosition.MarkPx),
			PositionSide: positionSide,
			PositionRisk: &toGlobalPositionRisk([]okexapi.Position{futuresPosition})[0],
			Symbol:       symbol,
		}
	}

	return retFuturesPositions
}

func toGlobalPositionSide(positionSide okexapi.PosSide) types.PositionType {
	switch positionSide {
	case okexapi.PosSideLong:
		return types.PositionLong
	case okexapi.PosSideShort:
		return types.PositionShort
	}
	return types.PositionType(positionSide)
}

func toGlobalPositionRisk(positions []okexapi.Position) []types.PositionRisk {
	retPositions := make([]types.PositionRisk, len(positions))
	for i, position := range positions {
		retPositions[i] = types.PositionRisk{
			Leverage:       position.Lever,
			Symbol:         toGlobalSymbol(position.InstId),
			PositionAmount: position.Pos,
			PositionSide:   toGlobalPositionSide(okexapi.PosSide(position.PosSide)),
			EntryPrice:     position.AvgPx,
			MarkPrice:      position.MarkPx,
			BreakEvenPrice: position.BePx,
			InitialMargin:  fixedpoint.MustNewFromString(position.Imr),
			MaintMargin:    fixedpoint.MustNewFromString(position.Mmr),
			UnrealizedPnL:  fixedpoint.MustNewFromString(position.Upl),
			Notional:       fixedpoint.MustNewFromString(position.NotionalUsd),
			MarginAsset:    position.Ccy,
			Adl:            position.Adl,
			UpdateTime:     position.UpdatedTime,
		}
	}
	return retPositions
}
