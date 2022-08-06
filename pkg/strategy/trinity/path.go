package trinity

import (
	"fmt"

	"github.com/c9s/bbgo/pkg/types"
)

type Path struct {
	marketA, marketB, marketC *ArbMarket
	dirA, dirB, dirC          int
}

func (p *Path) solveDirection() error {
	// check if we should reverse the rate
	// ETHUSDT -> ETHBTC
	if p.marketA.QuoteCurrency == p.marketB.BaseCurrency || p.marketA.QuoteCurrency == p.marketB.QuoteCurrency {
		p.dirA = 1
	} else if p.marketA.BaseCurrency == p.marketB.BaseCurrency || p.marketA.BaseCurrency == p.marketB.QuoteCurrency {
		p.dirA = -1
	} else {
		return fmt.Errorf("marketA and marketB is not related")
	}

	if p.marketB.QuoteCurrency == p.marketC.BaseCurrency || p.marketB.QuoteCurrency == p.marketC.QuoteCurrency {
		p.dirB = 1
	} else if p.marketB.BaseCurrency == p.marketC.BaseCurrency || p.marketB.BaseCurrency == p.marketC.QuoteCurrency {
		p.dirB = -1
	} else {
		return fmt.Errorf("marketB and marketC is not related")
	}

	if p.marketC.QuoteCurrency == p.marketA.BaseCurrency || p.marketC.QuoteCurrency == p.marketA.QuoteCurrency {
		p.dirC = 1
	} else if p.marketC.BaseCurrency == p.marketA.BaseCurrency || p.marketC.BaseCurrency == p.marketA.QuoteCurrency {
		p.dirC = -1
	} else {
		return fmt.Errorf("marketC and marketA is not related")
	}

	return nil
}

func (p *Path) Ready() bool {
	return !(p.marketA.bestAsk.Price.IsZero() || p.marketA.bestBid.Price.IsZero() ||
		p.marketB.bestAsk.Price.IsZero() || p.marketB.bestBid.Price.IsZero() ||
		p.marketC.bestAsk.Price.IsZero() || p.marketC.bestBid.Price.IsZero())
}

func (p *Path) String() string {
	return p.marketA.String() + " " + p.marketB.String() + " " + p.marketC.String()
}

func (p *Path) newForwardOrders(balances types.BalanceMap) []types.SubmitOrder {
	var transitingQuantity float64
	var transitingCurrency string
	var orders = make([]types.SubmitOrder, 3)

	initialBalance, transitingCurrency := p.marketA.getInitialBalance(balances, p.dirA)
	orderA, _ := p.marketA.newOrder(p.dirB, initialBalance.Float64())
	orders[0] = orderA

	q, c := orderA.Out()
	transitingQuantity, transitingCurrency = q.Float64(), c
	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)

	// orderB
	orderB, rateB := p.marketB.newOrder(p.dirB, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateB)

	q, c = orderB.Out()
	transitingQuantity, transitingCurrency = q.Float64(), c
	log.Infof("transiting quantity %f %s", transitingQuantity, transitingCurrency)
	orders[1] = orderB

	orderC, rateC := p.marketC.newOrder(p.dirC, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateC)

	q, c = orderC.Out()
	orders[2] = orderC

	return orders
}
