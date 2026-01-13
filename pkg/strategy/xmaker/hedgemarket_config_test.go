package xmaker

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func TestResolveQuotingDepthInQuote(t *testing.T) {
	price := fixedpoint.NewFromFloat(30000)

	t.Run("prefer in-quote when set", func(t *testing.T) {
		c := &HedgeMarketConfig{
			QuotingDepthInQuote: fixedpoint.NewFromFloat(500),
			QuotingDepth:        fixedpoint.NewFromFloat(2), // should be ignored
		}
		got := c.ResolveQuotingDepthInQuote(price)
		want := fixedpoint.NewFromFloat(500)
		if got.Compare(want) != 0 {
			t.Fatalf("got %s, want %s", got.String(), want.String())
		}
	})

	t.Run("derive in-quote from base when only base set", func(t *testing.T) {
		c := &HedgeMarketConfig{
			QuotingDepth: fixedpoint.NewFromFloat(2),
		}
		got := c.ResolveQuotingDepthInQuote(price)
		want := fixedpoint.NewFromFloat(60000)
		if got.Compare(want) != 0 {
			t.Fatalf("got %s, want %s", got.String(), want.String())
		}
	})

	t.Run("zero when no config", func(t *testing.T) {
		c := &HedgeMarketConfig{}
		got := c.ResolveQuotingDepthInQuote(price)
		if !got.IsZero() {
			t.Fatalf("expected zero, got %s", got.String())
		}
	})

	t.Run("zero when price is zero and only base set", func(t *testing.T) {
		c := &HedgeMarketConfig{QuotingDepth: fixedpoint.NewFromFloat(1)}
		got := c.ResolveQuotingDepthInQuote(fixedpoint.Zero)
		if !got.IsZero() {
			t.Fatalf("expected zero, got %s", got.String())
		}
	})
}

func TestResolveQuotingDepthInBase(t *testing.T) {
	price := fixedpoint.NewFromFloat(25000)

	t.Run("prefer base when set", func(t *testing.T) {
		c := &HedgeMarketConfig{
			QuotingDepth:        fixedpoint.NewFromFloat(3),
			QuotingDepthInQuote: fixedpoint.NewFromFloat(1000), // ignored
		}
		got := c.ResolveQuotingDepthInBase(price)
		want := fixedpoint.NewFromFloat(3)
		if got.Compare(want) != 0 {
			t.Fatalf("got %s, want %s", got.String(), want.String())
		}
	})

	t.Run("derive base from in-quote when only in-quote set", func(t *testing.T) {
		c := &HedgeMarketConfig{
			QuotingDepthInQuote: fixedpoint.NewFromFloat(50000),
		}
		got := c.ResolveQuotingDepthInBase(price)
		want := fixedpoint.NewFromFloat(2)
		if got.Compare(want) != 0 {
			t.Fatalf("got %s, want %s", got.String(), want.String())
		}
	})

	t.Run("zero when no config", func(t *testing.T) {
		c := &HedgeMarketConfig{}
		got := c.ResolveQuotingDepthInBase(price)
		if !got.IsZero() {
			t.Fatalf("expected zero, got %s", got.String())
		}
	})

	t.Run("zero when price is zero and only in-quote set", func(t *testing.T) {
		c := &HedgeMarketConfig{QuotingDepthInQuote: fixedpoint.NewFromFloat(1)}
		got := c.ResolveQuotingDepthInBase(fixedpoint.Zero)
		if !got.IsZero() {
			t.Fatalf("expected zero, got %s", got.String())
		}
	})
}
