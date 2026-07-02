package xfundingv2

import (
	"errors"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

const annualFundingHours = 24 * 365

// AnnualizedRate converts a per-period funding rate to annualized rate
func AnnualizedRate(fundingRate fixedpoint.Value, fundingIntervalHours int) fixedpoint.Value {
	numFundingPeriods := int64(annualFundingHours / fundingIntervalHours)
	return fundingRate.Mul(fixedpoint.NewFromInt(numFundingPeriods))
}

type CostEstimator struct {
	// targetPosition is the open position size in futures (can be positive or negative)
	// the open position size in spot is always the same but with opposite sign
	targetPosition fixedpoint.Value

	futuresFeeRate, spotFeeRate types.ExchangeFee
}

func NewCostEstimator() *CostEstimator {
	return &CostEstimator{}
}

func (c *CostEstimator) SetFuturesFeeRate(feeRate types.ExchangeFee) *CostEstimator {
	c.futuresFeeRate = feeRate
	return c
}

func (c *CostEstimator) SetSpotFeeRate(feeRate types.ExchangeFee) *CostEstimator {
	c.spotFeeRate = feeRate
	return c
}

func (c *CostEstimator) SetTargetPosition(position fixedpoint.Value) *CostEstimator {
	c.targetPosition = position
	return c
}

func (c *CostEstimator) GetFuturesFeeRate() types.ExchangeFee {
	return c.futuresFeeRate
}

func (c *CostEstimator) GetSpotFeeRate() types.ExchangeFee {
	return c.spotFeeRate
}

// EstimatedCost represents the estimated cost of a transaction, either entry or exit
type EstimatedCost struct {
	FuturesPosition fixedpoint.Value
	SpotFee         fixedpoint.Value
	FuturesFee      fixedpoint.Value
	SpreadPnL       fixedpoint.Value
}

// TotalFeeCost returns the total trading fee cost (spot fee + futures fee) in quote currency
func (e *EstimatedCost) TotalFeeCost() fixedpoint.Value {
	return e.SpotFee.Add(e.FuturesFee)
}

// Spread returns the price difference between spot and futures
// i.e spread = spot price - futures price
func (e *EstimatedCost) Spread() fixedpoint.Value {
	return e.SpreadPnL.Div(e.FuturesPosition)
}

// EstimateEntryCost calculates the cost of entering the position
func (c *CostEstimator) EstimateEntryCost(isMaker bool, spotOrderBook, futuresOrderBook types.OrderBook) (EstimatedCost, error) {
	if c.targetPosition.IsZero() {
		return EstimatedCost{}, nil
	}
	var spotPV, futuresPV types.PriceVolume
	var spotOk, futuresOk bool
	if c.targetPosition.Sign() < 0 {
		// short futures, long spot
		// buy spot at best ask
		spotPV, spotOk = spotOrderBook.BestAsk()
		// sell futures at best bid
		futuresPV, futuresOk = futuresOrderBook.BestBid()
	} else {
		// long futures, short spot
		// sell spot at best bid
		spotPV, spotOk = spotOrderBook.BestBid()
		// buy futures at best ask
		futuresPV, futuresOk = futuresOrderBook.BestAsk()
	}

	if !spotOk || !futuresOk {
		return EstimatedCost{}, errors.New("order book data is not ready yet")
	}

	return c.estimateCost(isMaker, spotPV.Price, futuresPV.Price), nil
}

func (c *CostEstimator) EstimateExitCost(isMaker bool, spotOrderBook, futuresOrderBook types.OrderBook) (EstimatedCost, error) {
	if c.targetPosition.IsZero() {
		return EstimatedCost{}, nil
	}

	var spotPV, futuresPV types.PriceVolume
	var spotOk, futuresOk bool
	if c.targetPosition.Sign() < 0 {
		// long futures, short spot to close
		spotPV, spotOk = spotOrderBook.BestBid()          // sell spot at best bid
		futuresPV, futuresOk = futuresOrderBook.BestAsk() // buy futures at best ask
	} else {
		// short futures, long spot to close
		spotPV, spotOk = spotOrderBook.BestAsk()          // buy spot at best ask
		futuresPV, futuresOk = futuresOrderBook.BestBid() // sell futures at best bid
	}

	if !spotOk || !futuresOk {
		return EstimatedCost{}, errors.New("order book data is not ready yet")
	}

	return c.estimateCost(isMaker, spotPV.Price, futuresPV.Price), nil
}

func (c *CostEstimator) estimateCost(isMaker bool, spotPrice, futuresPrice fixedpoint.Value) EstimatedCost {
	priceSpread := spotPrice.Sub(futuresPrice)
	// note that the c.targetPosition can be positive or negative, the spread PnL is hence:
	// let positoinSize = c.targetPosition.Abs()
	// spreadPnL = (spotPrice - futuresPrice) * positoinSize if c.targetPosition > 0 (long futures, short spot)
	// spreadPnL = (futuresPrice - spotPrice) * positoinSize if c.targetPosition < 0 (short futures, long spot)
	// then we have spreadPnL = (spotPrice - futuresPrice) * c.targetPosition, which works for both cases
	spreadPnL := priceSpread.Mul(c.targetPosition)

	var spotFeeRate, futuresFeeRate fixedpoint.Value
	if isMaker {
		spotFeeRate = c.spotFeeRate.MakerFeeRate
		futuresFeeRate = c.futuresFeeRate.MakerFeeRate
	} else {
		spotFeeRate = c.spotFeeRate.TakerFeeRate
		futuresFeeRate = c.futuresFeeRate.TakerFeeRate
	}

	// calculate fees in quote currency
	positionSize := c.targetPosition.Abs()
	spotFee := spotPrice.Mul(positionSize).Mul(spotFeeRate)
	futuresFee := futuresPrice.Mul(positionSize).Mul(futuresFeeRate)

	return EstimatedCost{
		FuturesPosition: c.targetPosition,
		SpotFee:         spotFee,
		FuturesFee:      futuresFee,
		SpreadPnL:       spreadPnL,
	}
}
