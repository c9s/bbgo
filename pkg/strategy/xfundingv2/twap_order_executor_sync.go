package xfundingv2

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/types"
)

type TWAPExecutorSyncState struct {
	Config    TWAPWorkerConfig            `json:"config"`
	Market    types.Market                `json:"market"`
	IsFutures bool                        `json:"isFutures"`
	Orders    map[uint64]types.OrderQuery `json:"orders,omitempty"`
	Trades    map[uint64]types.Trade      `json:"trades,omitempty"`
}

func (o *TWAPExecutor) Initialize(ctx context.Context, s *Strategy) error {
	o.SetLogger(s.logger)
	o.dryRun = s.DryRun
	o.ctx = ctx
	var session *bbgo.ExchangeSession
	var executor *bbgo.GeneralOrderExecutor
	if o.syncState.IsFutures {
		executor = s.futuresGeneralOrderExecutors[o.syncState.Market.Symbol]
		session = s.futuresSession
	} else {
		executor = s.spotGeneralOrderExecutors[o.syncState.Market.Symbol]
		session = s.spotSession
	}
	if executor == nil {
		return errors.New("[TWAPExecutor] futures general order executor not found for market: " + o.syncState.Market.Symbol)
	}
	o.executor = executor

	// sync market
	if market, ok := session.Market(o.syncState.Market.Symbol); !ok {
		return errors.New("[TWAPExecutor] market not found in session: " + o.syncState.Market.Symbol)
	} else {
		o.syncState.Market = market
	}
	// set order query service
	if service, ok := session.Exchange.(types.ExchangeOrderQueryService); ok {
		o.exchange = service
	} else {
		return errors.New("[TWAPExecutor] session exchange does not implement ExchangeOrderQueryService")
	}
	// sync orders/trades
	orderStore := executor.OrderStore()
	for _, query := range o.syncState.Orders {
		order, err := o.exchange.QueryOrder(o.ctx, query)
		if err != nil || order == nil {
			s.logger.WithError(err).Warnf("[TWAPExecutor] failed to query order via RESTful API: %v", query)
			order, err = s.Environment.TradeService.QueryOrderFromDB(query)
		}
		if err != nil || order == nil {
			s.logger.WithError(err).Errorf("[TWAPExecutor] missing order detected on %s: %v", session.Name, query)
			continue
		}
		orderStore.Add(*order)

		if trades, err := s.Environment.TradeService.QueryOrderTradesFromDB(query); err == nil {
			s.logger.Infof("[TWAPExecutor] found %d trades for order from DB: %v", len(trades), query)
			for _, trade := range trades {
				o.syncState.Trades[trade.ID] = trade
			}
		} else {
			s.logger.WithError(err).Warnf("[TWAPExecutor] failed to query trades for order from DB: %v", query)
		}
		if trades, err := o.exchange.QueryOrderTrades(o.ctx, query); err == nil {
			s.logger.Infof("[TWAPExecutor] found %d trades for order via RESTful API: %v", len(trades), query)
			for _, trade := range trades {
				o.syncState.Trades[trade.ID] = trade
			}
		} else {
			s.logger.WithError(err).Warnf("[TWAPExecutor] failed to query trades for order via RESTful API: %v", query)
		}
	}
	return nil
}

func (o *TWAPExecutor) MarshalJSON() ([]byte, error) {
	return json.Marshal(&o.syncState)
}

func (o *TWAPExecutor) UnmarshalJSON(b []byte) error {
	stateData := TWAPExecutorSyncState{}
	if err := json.Unmarshal(b, &stateData); err != nil {
		return err
	}

	o.syncState = stateData
	if o.syncState.Orders == nil {
		o.syncState.Orders = make(map[uint64]types.OrderQuery)
	}
	if o.syncState.Trades == nil {
		o.syncState.Trades = make(map[uint64]types.Trade)
	}

	return nil
}
