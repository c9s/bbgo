package tri

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

func (p *Path) newOrders(balances types.BalanceMap, sign int) [3]types.SubmitOrder {
	var orders [3]types.SubmitOrder
	var transitingQuantity float64

	initialBalance, _ := p.marketA.getInitialBalance(balances, p.dirA*sign)
	orderA, _ := p.marketA.newOrder(p.dirA*sign, initialBalance.Float64())
	orders[0] = orderA

	q, _ := orderA.Out()
	transitingQuantity = q.Float64()

	// orderB
	orderB, rateB := p.marketB.newOrder(p.dirB*sign, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateB)

	q, _ = orderB.Out()
	transitingQuantity = q.Float64()
	orders[1] = orderB

	orderC, rateC := p.marketC.newOrder(p.dirC*sign, transitingQuantity)
	orders = adjustOrderQuantityByRate(orders, rateC)

	q, _ = orderC.Out()
	orders[2] = orderC

	orders[0].Quantity = p.marketA.market.TruncateQuantity(orders[0].Quantity)
	orders[1].Quantity = p.marketB.market.TruncateQuantity(orders[1].Quantity)
	orders[2].Quantity = p.marketC.market.TruncateQuantity(orders[2].Quantity)
	return orders
}
