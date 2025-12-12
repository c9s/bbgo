package xmaker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const debtQuotaCacheDuration = 30 * time.Second

type DebtQuotaWorker struct {
	logger logrus.FieldLogger

	leverage       fixedpoint.Value
	minMarginLevel fixedpoint.Value
}

func (w *DebtQuotaWorker) calculateDebtQuota(ctx context.Context, session *bbgo.ExchangeSession) fixedpoint.Value {
	minMarginLevel := fixedpoint.NewFromFloat(1.01)

	// hedgeAccount := session.GetAccount()
	// bufMinMarginLevel := minMarginLevel.Mul(fixedpoint.NewFromFloat(1.005))

	accountValueCalculator := session.GetAccountValueCalculator()
	marketValue := accountValueCalculator.MarketValue()
	debtValue := accountValueCalculator.DebtValue()
	netValueInUsd := accountValueCalculator.NetValue()
	totalValue := accountValueCalculator.MarketValue()
	// marginInfoUpdater := session.GetMarginInfoUpdater()

	// sourceMarket := s.hedgeMarket
	w.logger.Infof(
		"account net value in usd: %f, debt value in usd: %f, total value in usd: %f",
		netValueInUsd.Float64(),
		debtValue.Float64(),
		marketValue.Float64(),
	)

	defaultMmr := calculateDefaultMMR(w.leverage)

	debtCap := totalValue.Div(minMarginLevel).Div(defaultMmr)
	marginLevel := totalValue.Div(debtValue).Div(defaultMmr)

	w.logger.Infof(
		"calculateDebtQuota: debtCap=%f, debtValue=%f currentMarginLevel=%f mmr=%f",
		debtCap.Float64(),
		debtValue.Float64(),
		marginLevel.Float64(),
		defaultMmr.Float64(),
	)

	debtQuota := debtCap.Sub(debtValue)
	if debtQuota.Sign() < 0 {
		return fixedpoint.Zero
	}

	return debtQuota
}

type DebtQuota struct {
	AmountInQuote fixedpoint.Value
}

func (w *DebtQuotaWorker) Run(ctx context.Context, sesWorker *SessionWorker) {
	session := sesWorker.session

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			debtQuota := w.calculateDebtQuota(ctx, session)
			sesWorker.SetValue(&DebtQuota{
				AmountInQuote: debtQuota,
			})
		}
	}
}

// calculateDefaultMMR calculates the default maintenance margin ratio based on leverage.
func calculateDefaultMMR(leverage fixedpoint.Value) fixedpoint.Value {
	defaultMmr := fixedpoint.NewFromFloat(9.0 * 0.01)
	if leverage.Compare(fixedpoint.NewFromFloat(10.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(5.0 * 0.01) // 5%
	} else if leverage.Compare(fixedpoint.NewFromFloat(5.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(9.0 * 0.01) // 9%
	} else if leverage.Compare(fixedpoint.NewFromFloat(3.0)) >= 0 {
		defaultMmr = fixedpoint.NewFromFloat(10.0 * 0.01) // 10%
	}
	return defaultMmr
}

// margin level = totalValue / totalDebtValue * MMR (maintenance margin ratio)
// on binance:
// - MMR with 10x leverage = 5%
// - MMR with 5x leverage = 9%
// - MMR with 3x leverage = 10%
func (s *Strategy) calculateDebtQuota(totalValue, debtValue, minMarginLevel, leverage fixedpoint.Value) fixedpoint.Value {
	now := time.Now()
	if s.debtQuotaCache != nil {
		if v, ok := s.debtQuotaCache.Get(now); ok {
			return v
		}
	}

	if minMarginLevel.IsZero() || totalValue.IsZero() {
		return fixedpoint.Zero
	}

	defaultMmr := calculateDefaultMMR(leverage)

	debtCap := totalValue.Div(minMarginLevel).Div(defaultMmr)
	marginLevel := totalValue.Div(debtValue).Div(defaultMmr)

	s.logger.Infof(
		"calculateDebtQuota: debtCap=%f, debtValue=%f currentMarginLevel=%f mmr=%f",
		debtCap.Float64(),
		debtValue.Float64(),
		marginLevel.Float64(),
		defaultMmr.Float64(),
	)

	debtQuota := debtCap.Sub(debtValue)
	if debtQuota.Sign() < 0 {
		return fixedpoint.Zero
	}

	if s.debtQuotaCache == nil {
		s.debtQuotaCache = fixedpoint.NewExpirable(debtQuota, now.Add(debtQuotaCacheDuration))
	} else {
		s.debtQuotaCache.Set(debtQuota, now.Add(debtQuotaCacheDuration))
	}

	return debtQuota
}
