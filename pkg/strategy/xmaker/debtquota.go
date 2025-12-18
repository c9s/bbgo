package xmaker

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/bbgo/sessionworker"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/risk"
)

const debtQuotaCacheDuration = 30 * time.Second

type DebtQuotaWorker struct {
	logger logrus.FieldLogger

	leverage       fixedpoint.Value
	minMarginLevel fixedpoint.Value
	maxLeverage    fixedpoint.Value

	interval time.Duration
}

type DebtQuotaResult struct {
	AmountInQuote fixedpoint.Value

	DebtValue       fixedpoint.Value
	NetValueInUsd   fixedpoint.Value
	TotalValueInUsd fixedpoint.Value
	MarginLevel     fixedpoint.Value
}

func (w *DebtQuotaWorker) calculate(ctx context.Context, session *bbgo.ExchangeSession) *DebtQuotaResult {
	minMarginLevel := fixedpoint.NewFromFloat(1.01)

	// hedgeAccount := session.GetAccount()
	// bufMinMarginLevel := minMarginLevel.Mul(fixedpoint.NewFromFloat(1.005))

	accountValueCalculator := session.GetAccountValueCalculator()
	marketValue := accountValueCalculator.MarketValue()
	debtValueInUsd := accountValueCalculator.DebtValue()
	netValueInUsd := accountValueCalculator.NetValue()
	totalValue := accountValueCalculator.MarketValue()
	// marginInfoUpdater := session.GetMarginInfoUpdater()

	// sourceMarket := s.hedgeMarket
	w.logger.Infof(
		"account net value in usd: %f, debt value in usd: %f, total value in usd: %f",
		netValueInUsd.Float64(),
		debtValueInUsd.Float64(),
		marketValue.Float64(),
	)

	defaultMmr := risk.DefaultMaintenanceMarginRatio(w.leverage)

	debtCap := totalValue.Div(minMarginLevel).Div(defaultMmr)
	marginLevel := totalValue.Div(debtValueInUsd).Div(defaultMmr)
	debtQuota := debtCap.Sub(debtValueInUsd)

	w.logger.Infof(
		"calculateDebtQuota: debtQuota=%f debtCap=%f, debtValueInUsd=%f currentMarginLevel=%f mmr=%f",
		debtQuota.Float64(),
		debtCap.Float64(),
		debtValueInUsd.Float64(),
		marginLevel.Float64(),
		defaultMmr.Float64(),
	)

	// preventing negative debt quota
	debtQuota = fixedpoint.Max(debtQuota, fixedpoint.Zero)

	if w.maxLeverage.Sign() > 0 {
		maximumValueInUsd := netValueInUsd.Mul(w.maxLeverage)
		leverageQuotaInUsd := maximumValueInUsd.Sub(debtValueInUsd)
		w.logger.Infof(
			"account maximum leveraged value in usd: %f (%f x), quota in usd: %f, min margin level quota: %f",
			maximumValueInUsd.Float64(),
			w.maxLeverage.Float64(),
			leverageQuotaInUsd.Float64(),
			debtQuota.Float64(),
		)

		debtQuota = fixedpoint.Min(debtQuota, leverageQuotaInUsd)
	}

	return &DebtQuotaResult{
		AmountInQuote:   debtQuota,
		DebtValue:       debtValueInUsd,
		NetValueInUsd:   netValueInUsd,
		TotalValueInUsd: totalValue,
		MarginLevel:     marginLevel,
	}
}

func (w *DebtQuotaWorker) Run(ctx context.Context, sesWorker *sessionworker.Handle) {
	session := sesWorker.Session()

	if w.interval == 0 {
		w.interval = 5 * time.Second
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rst := w.calculate(ctx, session)
			sesWorker.SetValue(rst)
		}
	}
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

	defaultMmr := risk.DefaultMaintenanceMarginRatio(leverage)

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
