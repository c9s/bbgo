package bfxfunding

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex"
	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bfxfunding"

// usdMinOfferAmount is the min funding offer amount of Bitfinex for the USD currencies
const usdMinOfferAmount = 150.0

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Strategy lends the funding wallet capital on Bitfinex in two tiers:
//   - the high rate tier parks a part of the capital in the high rate zone found from the public funding trade history,
//     with the loan period that the high rate trades are usually filled with.
//   - the channel tier places the rest at the upper edge of the normal rate channel to keep the capital utilized.
//
// Funding offers are only filled when a borrower's bid matches the rate almost exactly,
// so the rates and the periods are all derived from where the trades were actually filled.
type Strategy struct {
	Environment *bbgo.Environment

	// Currency is the funding symbol, e.g. fUST, fUSD, fBTC, fETH
	Currency string `json:"currency"`

	// Amount caps the capital used by the strategy, zero means the whole funding wallet
	Amount fixedpoint.Value `json:"amount"`

	// MinRate is the min daily rate of every offer
	MinRate fixedpoint.Value `json:"minRate"`

	// Lookback is the time range of the public funding trade history used to find the high rate zone
	Lookback types.Duration `json:"lookback"`

	RecomputeInterval types.Duration `json:"recomputeInterval"`

	// MinOfferAmount is the min amount of an offer, Bitfinex requires about 150 USD equivalent
	MinOfferAmount fixedpoint.Value `json:"minOfferAmount"`

	// MaxOfferAmount splits the tier amount into offers of at most this amount, zero means no split
	MaxOfferAmount fixedpoint.Value `json:"maxOfferAmount"`

	HighRate HighRateConfig `json:"highRate"`
	Channel  ChannelConfig  `json:"channel"`

	// KeepOffersOnStart keeps the active offers on start, otherwise they are canceled so that
	// the offers are always placed by the current analysis.
	KeepOffersOnStart bool `json:"keepOffersOnStart"`

	CancelOffersOnShutdown bool `json:"cancelOffersOnShutdown"`

	// DryRun runs the analysis and logs the planned offers without canceling or submitting any offer.
	// The capital is the configured amount, or the funding wallet balance when the amount is not set,
	// so the dry run works without the API credentials when the amount is set.
	DryRun bool `json:"dryRun"`

	// publicOnly is true when the session has no credentials
	publicOnly bool

	exchange *bitfinex.Exchange
	stream   *bitfinex.Stream
	client   *bfxapi.Client

	store   *TradeStore
	syncer  *TradeSyncer
	tracker *offerTracker

	// trades is the public funding trade history within the lookback, sorted by time.
	// it's only accessed by the recompute loop goroutine.
	trades []FundingTrade

	logger logrus.FieldLogger
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Currency)
}

func (s *Strategy) Defaults() error {
	if s.Currency == "" {
		s.Currency = "fUST"
	}

	if s.MinRate.IsZero() {
		s.MinRate = fixedpoint.NewFromFloat(0.01 * 0.01)
	}

	if s.Lookback == 0 {
		s.Lookback = types.Duration(90 * 24 * time.Hour)
	}

	if s.RecomputeInterval == 0 {
		s.RecomputeInterval = types.Duration(15 * time.Minute)
	}

	if s.MinOfferAmount.IsZero() && isUSDFunding(s.Currency) {
		s.MinOfferAmount = fixedpoint.NewFromFloat(usdMinOfferAmount)
	}

	if s.HighRate.Percentile == 0 {
		s.HighRate.Percentile = 0.95
	}

	if s.HighRate.CeilPercentile == 0 {
		s.HighRate.CeilPercentile = 0.995
	}

	if s.HighRate.MaxRatio == 0 {
		s.HighRate.MaxRatio = 0.4
	}

	if s.HighRate.VolumeShare == 0 {
		s.HighRate.VolumeShare = 0.05
	}

	if s.HighRate.Levels == 0 {
		s.HighRate.Levels = 3
	}

	if s.HighRate.RezoneTolerance == 0 {
		s.HighRate.RezoneTolerance = 0.1
	}

	if s.Channel.Window == 0 {
		s.Channel.Window = types.Duration(24 * time.Hour)
	}

	if s.Channel.LowerPercentile == 0 {
		s.Channel.LowerPercentile = 0.2
	}

	if s.Channel.UpperPercentile == 0 {
		s.Channel.UpperPercentile = 0.8
	}

	if s.Channel.RepriceTolerance == 0 {
		s.Channel.RepriceTolerance = 0.05
	}

	if s.Channel.PeriodRateTolerance == 0 {
		s.Channel.PeriodRateTolerance = 0.1
	}

	return nil
}

func (s *Strategy) Validate() error {
	if !strings.HasPrefix(s.Currency, "f") {
		return fmt.Errorf("currency must be a funding symbol like fUST, got %q", s.Currency)
	}

	if s.Amount.Sign() < 0 {
		return fmt.Errorf("amount must not be negative")
	}

	if s.MinRate.Sign() <= 0 {
		return fmt.Errorf("minRate must be greater than 0")
	}

	if s.MinOfferAmount.Sign() <= 0 {
		return fmt.Errorf("minOfferAmount is required for %s", s.Currency)
	}

	if s.MaxOfferAmount.Sign() > 0 && s.MaxOfferAmount.Compare(s.MinOfferAmount) < 0 {
		return fmt.Errorf("maxOfferAmount must not be less than minOfferAmount")
	}

	if s.Channel.Window.Duration() > s.Lookback.Duration() {
		return fmt.Errorf("channel.window must not be longer than lookback")
	}

	for name, v := range map[string]float64{
		"highRate.percentile":         s.HighRate.Percentile,
		"highRate.ceilPercentile":     s.HighRate.CeilPercentile,
		"highRate.maxRatio":           s.HighRate.MaxRatio,
		"highRate.volumeShare":        s.HighRate.VolumeShare,
		"channel.lowerPercentile":     s.Channel.LowerPercentile,
		"channel.upperPercentile":     s.Channel.UpperPercentile,
		"channel.repriceTolerance":    s.Channel.RepriceTolerance,
		"channel.periodRateTolerance": s.Channel.PeriodRateTolerance,
		"highRate.rezoneTolerance":    s.HighRate.RezoneTolerance,
	} {
		if v <= 0 || v > 1 {
			return fmt.Errorf("%s must be in (0, 1], got %f", name, v)
		}
	}

	if s.HighRate.CeilPercentile < s.HighRate.Percentile {
		return fmt.Errorf("highRate.ceilPercentile must not be less than highRate.percentile")
	}

	if s.Channel.UpperPercentile <= s.Channel.LowerPercentile {
		return fmt.Errorf("channel.upperPercentile must be greater than channel.lowerPercentile")
	}

	if s.HighRate.Levels <= 0 {
		return fmt.Errorf("highRate.levels must be greater than 0")
	}

	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = logrus.WithFields(logrus.Fields{"strategy": s.InstanceID(), "currency": s.Currency})
	s.tracker = newOfferTracker()
	return nil
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// the funding offer events come from the authenticated user data stream, no subscription is needed
}

func (s *Strategy) handleFundingOfferSnapshot(e *bfxapi.FundingOfferSnapshotEvent) {
	s.logger.Debugf("funding offer snapshot: %d offers", len(e.Offers))
}

func (s *Strategy) handleFundingOfferUpdate(e *bfxapi.FundingOfferUpdateEvent) {
	if e.Symbol != s.Currency {
		return
	}

	if strings.HasPrefix(e.Status, "ACTIVE") || strings.HasPrefix(e.Status, "PARTIALLY") {
		return
	}

	if o, ok := s.tracker.Get(e.OfferID); ok {
		s.logger.Infof("%s funding offer %d %s: %s @ %s for %dd",
			o.Tier, e.OfferID, e.Status, e.AmountOrig, e.Rate, e.Period)
	}

	s.tracker.Remove(e.OfferID)
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if session.ExchangeName != types.ExchangeBitfinex {
		return fmt.Errorf("bfxfunding strategy only works with bitfinex exchange")
	}

	ex, ok := session.Exchange.(*bitfinex.Exchange)
	if !ok {
		return fmt.Errorf("exchange is not bitfinex exchange")
	}

	s.exchange = ex
	s.publicOnly = session.PublicOnly
	s.client = s.exchange.GetApiClient()
	s.stream = session.UserDataStream.(*bitfinex.Stream)
	s.syncer = NewTradeSyncer(s.client, s.Currency, s.logger)

	if s.Environment != nil && s.Environment.DatabaseService != nil && s.Environment.DatabaseService.DB != nil {
		s.store = NewTradeStore(s.Environment.DatabaseService.DB)
		if err := s.store.Check(ctx); err != nil {
			s.logger.WithError(err).Warnf("funding trade table is not available, add %q to database.extraMigrationPackages "+
				"to persist the history, the %s of history will be fetched on every start", ID, s.Lookback.Duration())
			s.store = nil
		}
	} else {
		s.logger.Warnf("database is not configured, the %s of funding trade history will be fetched on every start",
			s.Lookback.Duration())
	}

	s.stream.OnFundingOfferSnapshotEvent(s.handleFundingOfferSnapshot)
	s.stream.OnFundingOfferUpdateEvent(s.handleFundingOfferUpdate)

	s.logParameters()

	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		if s.CancelOffersOnShutdown && !s.DryRun {
			if err := s.cancelAllOffers(ctx); err != nil {
				s.logger.WithError(err).Error("unable to cancel funding offers on shutdown")
			}
		}
	})

	go s.run(ctx)
	return nil
}

func (s *Strategy) run(ctx context.Context) {
	if !s.KeepOffersOnStart && !s.DryRun {
		if err := s.cancelAllOffers(ctx); err != nil {
			s.logger.WithError(err).Error("unable to cancel the active funding offers on start")
		}
	}

	if err := s.loadTrades(ctx); err != nil {
		s.logger.WithError(err).Error("unable to load the funding trade history")
	}

	ticker := time.NewTicker(s.RecomputeInterval.Duration())
	defer ticker.Stop()

	for {
		if err := s.syncTrades(ctx); err != nil {
			s.logger.WithError(err).Error("unable to sync the funding trades")
		}

		if err := s.rebalance(ctx); err != nil {
			s.logger.WithError(err).Error("unable to rebalance the funding offers")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// loadTrades loads the stored funding trades within the lookback and moves the syncer cursor to the latest one
func (s *Strategy) loadTrades(ctx context.Context) error {
	if s.store == nil {
		return nil
	}

	trades, err := s.store.Query(ctx, s.Currency, time.Now().Add(-s.Lookback.Duration()))
	if err != nil {
		return err
	}

	s.trades = trades
	if n := len(trades); n > 0 {
		last := trades[n-1].Time
		var ids []int64
		for i := n - 1; i >= 0 && trades[i].Time.Equal(last); i-- {
			ids = append(ids, trades[i].ID)
		}

		s.syncer.SetCursor(last, ids)
		s.logger.Infof("loaded %d stored funding trades since %s", n, trades[0].Time)
	}

	return nil
}

// syncTrades fetches the new funding trades, stores them and drops the ones out of the lookback
func (s *Strategy) syncTrades(ctx context.Context) error {
	since := time.Now().Add(-s.Lookback.Duration())
	pages := 0
	n, err := s.syncer.Sync(ctx, since, func(trades []FundingTrade) error {
		if s.store != nil {
			if err := s.store.Insert(ctx, s.Currency, trades); err != nil {
				return fmt.Errorf("unable to store funding trades: %w", err)
			}
		}

		s.trades = append(s.trades, trades...)

		pages++
		if pages%10 == 0 {
			s.logger.Infof("syncing funding trades, %d pages fetched, cursor: %s", pages, trades[len(trades)-1].Time)
		}

		return nil
	})

	if n > 0 {
		s.logger.Infof("synced %d funding trades", n)
	}

	s.trimTrades(since)
	return err
}

func (s *Strategy) trimTrades(since time.Time) {
	i := 0
	for i < len(s.trades) && s.trades[i].Time.Before(since) {
		i++
	}

	if i == 0 {
		return
	}

	// copy the trades to release the underlying array
	s.trades = append([]FundingTrade(nil), s.trades[i:]...)
}

func (s *Strategy) offerLimits() OfferLimits {
	return OfferLimits{
		MinAmount: s.MinOfferAmount.Float64(),
		MaxAmount: s.MaxOfferAmount.Float64(),
		MinRate:   s.MinRate.Float64(),
	}
}

// rebalance re-runs the analysis, cancels the offers drifted from it and places the idle capital
func (s *Strategy) rebalance(ctx context.Context) error {
	now := time.Now()
	zone, err := AnalyzeHighRateZone(s.trades, now, s.Lookback.Duration(), s.HighRate)
	if err != nil {
		return fmt.Errorf("unable to analyze the high rate zone: %w", err)
	}

	channel, err := AnalyzeRateChannel(s.trades, now, s.Channel.Window.Duration(), s.Channel)
	if err != nil {
		return fmt.Errorf("unable to analyze the rate channel: %w", err)
	}

	s.logAnalysis(zone, channel)

	if s.DryRun {
		return s.dryRunPlan(ctx, zone, channel)
	}

	offers, err := s.queryActiveOffers(ctx)
	if err != nil {
		return err
	}

	active := make(map[int64]struct{}, len(offers))
	for _, o := range offers {
		active[o.ID] = struct{}{}
	}

	s.tracker.Retain(active)

	var keptHigh float64
	for _, o := range offers {
		tracked, ok := s.tracker.Get(o.ID)
		if !ok {
			// the offers not placed by this process are classified by the current zone
			tracked = trackedOffer{Tier: TierChannel}
			if o.Rate.Float64() >= zone.Floor {
				tracked = trackedOffer{Tier: TierHighRate, Floor: zone.Floor}
			}

			s.tracker.Add(o.ID, tracked)
		}

		if reason := s.driftReason(o, tracked, zone, channel); reason != "" {
			s.logger.Infof("canceling %s funding offer %d (%s @ %s for %sd): %s",
				tracked.Tier, o.ID, o.Amount, o.Rate, o.Period, reason)
			if err := s.cancelOffer(ctx, o.ID); err != nil {
				s.logger.WithError(err).Error("unable to cancel the drifted funding offer")
			}

			continue
		}

		if tracked.Tier == TierHighRate {
			keptHigh += o.Amount.Float64()
		}
	}

	// query the wallet after the cancellations so that the freed capital is included
	wallet, err := s.queryFundingWallet(ctx)
	if err != nil {
		return err
	}

	capital, idle := s.capitalOf(wallet, 0)

	highTarget := HighRateTarget(capital, zone, s.HighRate)
	specs := s.planOffers(idle, math.Max(highTarget-keptHigh, 0), zone, channel)

	s.logPlan(zone, allocation{Capital: capital, Idle: idle, HighTarget: highTarget, KeptHigh: keptHigh}, specs)

	for _, spec := range specs {
		offer, err := s.submitOffer(ctx, spec, zone.Floor)
		if err != nil {
			s.logger.WithError(err).Error("unable to submit the funding offer")
			continue
		}

		s.logger.Infof("submitted funding offer %d: %s", offer.ID, spec)
	}

	return nil
}

// dryRunPlan plans the offers as if the whole capital is idle and logs them without submitting
// With the credentials, the capital and the idle amount are computed from the funding wallet like the live mode,
// otherwise the configured amount is treated as idle capital.
func (s *Strategy) dryRunPlan(ctx context.Context, zone HighRateZone, channel RateChannel) error {
	capital, idle := s.Amount.Float64(), s.Amount.Float64()
	var keptHigh float64
	if !s.publicOnly {
		if wallet, freed, kept := s.logAccountState(ctx, zone, channel); wallet != nil {
			capital, idle = s.capitalOf(wallet, freed)
			keptHigh = kept
		}
	}

	if capital <= 0 && s.Amount.IsZero() {
		return fmt.Errorf("dry run needs the amount to be set when the funding wallet is not available")
	}

	highTarget := HighRateTarget(capital, zone, s.HighRate)
	specs := s.planOffers(idle, math.Max(highTarget-keptHigh, 0), zone, channel)
	s.logPlan(zone, allocation{Capital: capital, Idle: idle, HighTarget: highTarget, KeptHigh: keptHigh}, specs)
	return nil
}

// capitalOf returns the capital of the strategy and the idle amount to place from the funding wallet,
// freed is the amount of the offers to be canceled that is not reflected in the wallet available balance yet.
func (s *Strategy) capitalOf(wallet *bfxapi.Wallet, freed float64) (capital, idle float64) {
	balance := wallet.Balance.Float64()
	idle = wallet.AvailableBalance.Float64() + freed
	capital = balance
	if s.Amount.Sign() > 0 {
		capital = math.Min(balance, s.Amount.Float64())

		// the lent capital and the active offers are already in use
		used := balance - idle
		idle = math.Min(idle, capital-used)
	}

	return capital, math.Max(idle, 0)
}

// logAccountState logs the funding wallet and what the live mode would do to the active offers, read-only.
// It returns the wallet, the amount of the offers that would be canceled and the amount of the high rate offers kept.
func (s *Strategy) logAccountState(
	ctx context.Context, zone HighRateZone, channel RateChannel,
) (wallet *bfxapi.Wallet, freed, keptHigh float64) {
	wallet, err := s.queryFundingWallet(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("[DRY RUN] unable to query the funding wallet")
		return nil, 0, 0
	}

	lent := wallet.Balance.Sub(wallet.AvailableBalance)
	s.logger.Infof("[DRY RUN] funding wallet %s: balance %s, available %s, lent or offered %s, unsettled interest %s",
		wallet.Currency, wallet.Balance, wallet.AvailableBalance, lent, wallet.UnsettledInterest)

	offers, err := s.queryActiveOffers(ctx)
	if err != nil {
		s.logger.WithError(err).Warn("[DRY RUN] unable to query the active funding offers")
		return wallet, 0, 0
	}

	s.logger.Infof("[DRY RUN] %d active %s funding offers", len(offers), s.Currency)
	for _, o := range offers {
		action := "cancel (on start)"
		cancel := true
		if s.KeepOffersOnStart {
			cancel = false
			tracked := trackedOffer{Tier: TierChannel}
			if o.Rate.Float64() >= zone.Floor {
				tracked = trackedOffer{Tier: TierHighRate, Floor: zone.Floor}
			}

			action = fmt.Sprintf("keep as %s tier", tracked.Tier)
			if reason := s.driftReason(o, tracked, zone, channel); reason != "" {
				action = fmt.Sprintf("cancel as %s tier: %s", tracked.Tier, reason)
				cancel = true
			} else if tracked.Tier == TierHighRate {
				keptHigh += o.Amount.Float64()
			}
		}

		if cancel {
			freed += o.Amount.Float64()
		}

		s.logger.Infof("[DRY RUN]   offer %d: %s @ %s for %sd, status %s -> would %s",
			o.ID, o.Amount, formatRate(o.Rate.Float64()), o.Period, o.OfferStatus, action)
	}

	return wallet, freed, keptHigh
}

// planOffers places the high rate tier up to highAmount first, and the rest of the idle capital to the channel tier
func (s *Strategy) planOffers(idle, highAmount float64, zone HighRateZone, channel RateChannel) []OfferSpec {
	limits := s.offerLimits()
	specs := PlanHighRateOffers(math.Min(highAmount, idle), zone, limits)

	var planned float64
	for _, spec := range specs {
		planned += spec.Amount
	}

	return append(specs, PlanChannelOffers(idle-planned, channel, limits)...)
}

// driftReason returns why the offer should be re-placed, or an empty string if it should be kept
func (s *Strategy) driftReason(o bfxapi.FundingOffer, tracked trackedOffer, zone HighRateZone, channel RateChannel) string {
	rate := o.Rate.Float64()
	if rate < s.MinRate.Float64() {
		return fmt.Sprintf("rate is below the min rate %s", s.MinRate)
	}

	switch tracked.Tier {
	case TierHighRate:
		if tracked.Floor > 0 && math.Abs(zone.Floor-tracked.Floor)/tracked.Floor > s.HighRate.RezoneTolerance {
			return fmt.Sprintf("high rate zone floor moved from %s to %s", formatRate(tracked.Floor), formatRate(zone.Floor))
		}

	case TierChannel:
		edge := math.Max(channel.Upper, s.MinRate.Float64())
		if math.Abs(rate-edge)/edge > s.Channel.RepriceTolerance {
			return fmt.Sprintf("rate drifted from the channel edge %s", formatRate(edge))
		}

		if int(o.Period.Float64()) != channel.Period {
			return fmt.Sprintf("period changed to %dd", channel.Period)
		}
	}

	return ""
}

func isUSDFunding(symbol string) bool {
	switch symbol {
	case "fUSD", "fUST", "fUDC":
		return true
	}

	return false
}
