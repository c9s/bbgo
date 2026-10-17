package bfxfunding

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/c9s/bbgo/pkg/exchange/bitfinex/bfxapi"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

const (
	responseStatusSuccess = "SUCCESS"

	// Bitfinex accepts up to 8 decimals for the funding rate and amount
	offerPrecision = 8
)

// trackedOffer is the strategy side state of an active funding offer
type trackedOffer struct {
	Tier Tier

	// Floor is the high rate zone floor when a high rate offer was placed
	Floor float64
}

// offerTracker maps the active funding offers to the tiers they were placed for
type offerTracker struct {
	mu     sync.Mutex
	offers map[int64]trackedOffer
}

func newOfferTracker() *offerTracker {
	return &offerTracker{offers: make(map[int64]trackedOffer)}
}

func (t *offerTracker) Add(id int64, o trackedOffer) {
	t.mu.Lock()
	t.offers[id] = o
	t.mu.Unlock()
}

func (t *offerTracker) Get(id int64) (trackedOffer, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	o, ok := t.offers[id]
	return o, ok
}

func (t *offerTracker) Remove(id int64) {
	t.mu.Lock()
	delete(t.offers, id)
	t.mu.Unlock()
}

// Retain drops the offers that are not active anymore
func (t *offerTracker) Retain(active map[int64]struct{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for id := range t.offers {
		if _, ok := active[id]; !ok {
			delete(t.offers, id)
		}
	}
}

// walletCurrency converts the funding symbol to the wallet currency, e.g. fUST -> UST
func walletCurrency(symbol string) string {
	return strings.TrimPrefix(symbol, "f")
}

func (s *Strategy) queryFundingWallet(ctx context.Context) (*bfxapi.Wallet, error) {
	wallets, err := s.client.NewGetWalletsRequest().Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to query wallets: %w", err)
	}

	currency := walletCurrency(s.Currency)
	for _, w := range wallets {
		if w.Type == bfxapi.WalletTypeFunding && w.Currency == currency {
			return &w, nil
		}
	}

	return nil, fmt.Errorf("funding wallet of %s not found", currency)
}

func (s *Strategy) queryActiveOffers(ctx context.Context) ([]bfxapi.FundingOffer, error) {
	offers, err := s.client.Funding().NewGetActiveFundingOffersRequest().Symbol(s.Currency).Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to query active funding offers: %w", err)
	}

	return offers, nil
}

func (s *Strategy) submitOffer(ctx context.Context, spec OfferSpec, floor float64) (*bfxapi.FundingOffer, error) {
	amount := fixedpoint.NewFromFloat(spec.Amount).Round(offerPrecision, fixedpoint.Down)
	rate := fixedpoint.NewFromFloat(spec.Rate).Round(offerPrecision, fixedpoint.HalfUp)

	resp, err := s.client.Funding().NewSubmitFundingOfferRequest().
		Symbol(s.Currency).
		Amount(amount.String()).
		Rate(rate.String()).
		Period(spec.Period).
		OfferType(bfxapi.FundingOfferTypeLimit).
		Notify(false).
		Hidden(false).
		AutoRenew(false).
		Do(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to submit %s: %w", spec, err)
	}

	if resp.Status != responseStatusSuccess {
		return nil, fmt.Errorf("unable to submit %s: %s %s", spec, resp.Status, resp.Text)
	}

	offer := resp.FundingOffer
	s.tracker.Add(offer.ID, trackedOffer{Tier: spec.Tier, Floor: floor})
	return &offer, nil
}

func (s *Strategy) cancelOffer(ctx context.Context, id int64) error {
	resp, err := s.client.Funding().NewCancelFundingOfferRequest().Id(id).Do(ctx)
	if err != nil {
		return fmt.Errorf("unable to cancel funding offer %d: %w", id, err)
	}

	if resp.Status != responseStatusSuccess {
		text := ""
		if resp.Text != nil {
			text = *resp.Text
		}

		return fmt.Errorf("unable to cancel funding offer %d: %s %s", id, resp.Status, text)
	}

	s.tracker.Remove(id)
	return nil
}

func (s *Strategy) cancelAllOffers(ctx context.Context) error {
	offers, err := s.queryActiveOffers(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, o := range offers {
		if err := s.cancelOffer(ctx, o.ID); err != nil {
			errs = append(errs, err)
			continue
		}

		s.logger.Infof("canceled funding offer %d: %s @ %s for %sd", o.ID, o.Amount, o.Rate, o.Period)
	}

	if len(errs) > 0 {
		return fmt.Errorf("unable to cancel %d funding offers: %v", len(errs), errs)
	}

	return nil
}
