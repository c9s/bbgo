package dca3

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
)

func (s *Strategy) UpdateProfitStatsUntilSuccessful(ctx context.Context) error {
	var op = func() error {
		if updated, err := s.UpdateProfitStats(ctx); err != nil {
			return errors.Wrapf(err, "failed to update profit stats, please check it")
		} else if !updated {
			return fmt.Errorf("there is no round to update profit stats, please check it")
		}

		return nil
	}

	// exponential increased interval retry until success
	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 5 * time.Second
	bo.MaxInterval = 20 * time.Minute
	bo.MaxElapsedTime = 0

	return backoff.Retry(op, backoff.WithContext(bo, ctx))
}

// UpdateProfitStats will collect round from closed orders and emit update profit stats
// return true, nil -> there is at least one finished round and all the finished rounds we collect update profit stats successfully
// return false, nil -> there is no finished round!
// return true, error -> At least one round update profit stats successfully but there is error when collecting other rounds
func (s *Strategy) UpdateProfitStats(ctx context.Context) (bool, error) {
	s.logger.Info("update profit stats")
	rounds, err := s.collector.CollectRoundsFromOrderID(ctx, s.ProfitStats.FromOrderID)
	if err != nil {
		return false, errors.Wrapf(err, "failed to collect finish rounds from #%d", s.ProfitStats.FromOrderID)
	}

	var updated bool = false
	for _, round := range rounds {
		if len(round.TakeProfitOrders) == 0 {
			return updated, nil
		}

		roundPosition := types.NewPositionFromMarket(s.Market)
		trades, err := s.collector.CollectRoundTrades(ctx, round)
		if err != nil {
			return updated, errors.Wrapf(err, "failed to collect the trades of round")
		}

		for _, trade := range trades {
			s.ProfitStats.AddTrade(trade)
			roundPosition.AddTrade(trade)
			// s.logger.Infof("update profit stats from trade: %s\nposition: %s\nprofit state: %s", trade.String(), roundPosition.String(), s.ProfitStats.String())
			var sb strings.Builder
			sb.WriteString("update profit stats:\n")
			sb.WriteString("[---------------------- Trade ---------------------]\n")
			sb.WriteString(trade.String() + "\n")
			sb.WriteString("[-------------------- Position --------------------]\n")
			sb.WriteString(roundPosition.String() + "\n")
			sb.WriteString(s.ProfitStats.String())
			s.logger.Info(sb.String())
		}

		if roundPosition.GetBase().Compare(s.Market.MinQuantity) > 0 {
			// if there is still open position, it means this round is not finished
			return updated, nil
		}

		// update profit stats FromOrderID to make sure we will not collect duplicated rounds
		for _, order := range round.TakeProfitOrders {
			if order.OrderID >= s.ProfitStats.FromOrderID {
				s.ProfitStats.FromOrderID = order.OrderID + 1
			}
		}

		// update quote investment
		s.ProfitStats.QuoteInvestment = s.ProfitStats.QuoteInvestment.Add(s.ProfitStats.CurrentRoundProfit)
		s.logger.Infof("profit stats:\n%s", s.ProfitStats.String())

		// sync to persistence
		bbgo.Sync(ctx, s)
		updated = true

		// emit profit
		s.EmitProfit(s.ProfitStats)

		// make profit stats forward to new round
		s.ProfitStats.NewRound()
	}

	return updated, nil
}
