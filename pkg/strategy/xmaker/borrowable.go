package xmaker

import (
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

// borrowEligibilityWarnLimiter rate-limits the warning emitted when a hedge
// asset is not borrowable on the margin hedge exchange (borrowability is
// near-static, so the warning does not need to fire on every quote cycle).
var borrowEligibilityWarnLimiter = rate.NewLimiter(rate.Every(3*time.Minute), 1)

type BorrowableAssetResult struct {
	BaseBorrowable  bool
	QuoteBorrowable bool
}

type AssetsBorrowablity map[string]bool

func addBorrowableAssets(session *bbgo.ExchangeSession, market types.Market) {
	updater := session.GetMarginInfoUpdater()
	if updater == nil {
		return
	}
	updater.AddBorrowableAssets(market.BaseCurrency, market.QuoteCurrency)
}

// getBorrowableAssetResult reads the cached borrowability result for the given hedge market
func getBorrowableAssetResult(session *bbgo.ExchangeSession, market types.Market, logger logrus.FieldLogger) *BorrowableAssetResult {
	updater := session.GetMarginInfoUpdater()
	if updater == nil {
		return nil
	}
	baseBorrowable, baseFound := updater.GetMaxBorrowable(market.BaseCurrency)
	quoteBorrowable, quoteFound := updater.GetMaxBorrowable(market.QuoteCurrency)
	if !baseFound || !quoteFound {
		// return nil to disable the borrowablity check.
		logger.Warnf("at least one of the assets is missing from the borrowable assets result on %s: %s", session.Name, market.Symbol)
		return nil
	}

	return &BorrowableAssetResult{
		BaseBorrowable:  baseBorrowable.Sign() > 0,
		QuoteBorrowable: quoteBorrowable.Sign() > 0,
	}
}

func (s *Strategy) simpleHedgeBorrowableCheck(oriDisableMakerBid, oriDisableMakerAsk bool) (bool, bool) {
	// When hedging on a margin account, a maker bid hedge borrows the base
	// currency and a maker ask hedge borrows the quote currency. If the hedge
	// exchange currently disallows borrowing an asset, disable the affected maker
	// side so we stop quoting a hedge we cannot cover.
	disableMakerBid := oriDisableMakerBid
	disableMakerAsk := oriDisableMakerAsk
	if !s.hedgeSession.Margin || (disableMakerBid && disableMakerAsk) {
		// both sides are already disabled, no need to check borrowable assets
		return disableMakerBid, disableMakerAsk
	}

	if borrowable := getBorrowableAssetResult(s.hedgeSession, s.hedgeMarket, s.logger); borrowable != nil {
		account := s.hedgeSession.Account
		baseBalance, _ := account.Balance(s.hedgeMarket.BaseCurrency)
		quoteBalance, _ := account.Balance(s.hedgeMarket.QuoteCurrency)
		if !borrowable.BaseBorrowable && baseBalance.Available.IsZero() {
			// base is not borrowable (ask) and we have 0 balance -> not able to hedge maker bid orders -> disable maker bid orders
			s.logger.Warnf(
				"unable to hedge maker bid orders on %s, base borrowable: %v, base balance: %s",
				s.hedgeSession.ExchangeName, borrowable.BaseBorrowable, baseBalance.String(),
			)
			disableMakerBid = true
		}
		if !borrowable.QuoteBorrowable && quoteBalance.Available.IsZero() {
			// quote is not borrowable (bid) and we have 0 balance -> not able to hedge maker ask orders -> disable maker ask orders
			s.logger.Warnf(
				"unable to hedge maker ask orders on %s, quote borrowable: %v, quote balance: %s",
				s.hedgeSession.ExchangeName, borrowable.QuoteBorrowable, quoteBalance.String(),
			)
			disableMakerAsk = true
		}

		if borrowEligibilityWarnLimiter.Allow() {
			if !borrowable.BaseBorrowable {
				s.logger.Warnf(
					"%s base currency %s is not borrowable on %s, disabling maker bid orders...",
					s.Symbol, s.hedgeMarket.BaseCurrency, s.hedgeSession.ExchangeName,
				)
			}
			if !borrowable.QuoteBorrowable {
				s.logger.Warnf(
					"%s quote currency %s is not borrowable on %s, disabling maker ask orders...",
					s.Symbol, s.hedgeMarket.QuoteCurrency, s.hedgeSession.ExchangeName,
				)
			}
		}
	} else {
		s.logger.Warnf(
			"unable to get borrowable asset result on %s, no borrowable check performed",
			s.hedgeSession.ExchangeName,
		)
	}
	return disableMakerBid, disableMakerAsk
}

func (s *Strategy) splitHedgeBorrowableCheck(oriDisableMakerBid, oriDisableMakerAsk bool) (bool, bool) {
	disableMakerBid := oriDisableMakerBid
	disableMakerAsk := oriDisableMakerAsk
	if disableMakerBid && disableMakerAsk {
		// both sides are already disabled, no need to check borrowable assets
		return disableMakerBid, disableMakerAsk
	}

	// split hedge is enabled, we need to check there is at least one hedge session is available to place the hedge orders
	var availableAskSessions []string
	var availableBidSessions []string
	for _, hedgeMarket := range s.SplitHedge.hedgeMarketInstances {
		if !hedgeMarket.session.Margin {
			availableAskSessions = append(availableAskSessions, hedgeMarket.session.Name)
			availableBidSessions = append(availableBidSessions, hedgeMarket.session.Name)
			continue
		}
		if borrowable := getBorrowableAssetResult(hedgeMarket.session, hedgeMarket.market, s.logger); borrowable != nil {
			account := hedgeMarket.session.Account
			baseBalance, _ := account.Balance(hedgeMarket.market.BaseCurrency)
			quoteBalance, _ := account.Balance(hedgeMarket.market.QuoteCurrency)
			if borrowable.BaseBorrowable || baseBalance.Available.Sign() > 0 {
				// base borrowable on hedge market or we have available base balance (ask) -> we can hedge bid orders on maker market
				availableAskSessions = append(availableAskSessions, hedgeMarket.session.Name)
			} else {
				s.logger.Warnf(
					"unable to hedge maker bid orders on %s, base borrowable: %v, base balance: %s",
					hedgeMarket.session.Name, borrowable.BaseBorrowable, baseBalance.String(),
				)
			}
			if borrowable.QuoteBorrowable || quoteBalance.Available.Sign() > 0 {
				// quote borrowable on hedge market or we have available quote balance (bid) -> we can hedge ask orders on maker market
				availableBidSessions = append(availableBidSessions, hedgeMarket.session.Name)
			} else {
				s.logger.Warnf(
					"unable to hedge maker ask orders on %s, quote borrowable: %v, quote balance: %s",
					hedgeMarket.session.Name, borrowable.QuoteBorrowable, quoteBalance.String(),
				)
			}
		} else {
			// we cannot get the borrowable asset result, we assume the hedge market is available for both ask and bid
			s.logger.Warnf("unable to get borrowable asset result on %s, assume both assets are borrowable", hedgeMarket.session.Name)
			availableAskSessions = append(availableAskSessions, hedgeMarket.session.Name)
			availableBidSessions = append(availableBidSessions, hedgeMarket.session.Name)
		}
	}
	if len(availableAskSessions) == 0 {
		// no hedge session is available to place the ask orders -> can not hedge maker bid orders -> disable maker bid
		disableMakerBid = true
	}
	if len(availableBidSessions) == 0 {
		// no hedge session is available to place the bid orders -> can not hedge maker ask orders -> disable maker ask
		disableMakerAsk = true
	}

	if borrowEligibilityWarnLimiter.Allow() {
		if len(availableAskSessions) == 0 {
			s.logger.Warn("split hedge has no available hedge markets to place maker ask orders, disable maker ask quoting")
		}
		if len(availableBidSessions) == 0 {
			s.logger.Warn("split hedge has no available hedge markets to place maker bid orders, disable maker bid quoting")
		}
	}

	return disableMakerBid, disableMakerAsk
}
