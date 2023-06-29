# Risk Control
------------

### 1. Introduction

Two types of risk controls for strategies is created:
- Position-limit Risk Control (pkg/risk/riskcontrol/position.go)
- Circuit-break Risk Control (pkg/risk/riskcontrol/circuit_break.go)

### 2. Position-Limit Risk Control

Initialization:
```
    s.positionRiskControl = riskcontrol.NewPositionRiskControl(s.HardLimit, s.Quantity, s.orderExecutor.TradeCollector())
    s.positionRiskControl.OnReleasePosition(func(quantity fixedpoint.Value, side types.SideType) {
        createdOrders, err := s.orderExecutor.SubmitOrders(ctx, types.SubmitOrder{
            Symbol:   s.Symbol,
            Market:   s.Market,
            Side:     side,
            Type:     types.OrderTypeMarket,
            Quantity: quantity,
        })
        if err != nil {
            log.WithError(err).Errorf("failed to submit orders")
            return
        }
        log.Infof("created orders: %+v", createdOrders)
    })
```

Strategy should provide OnReleasePosition callback, which will be called when position (positive or negative) is over hard limit.

Modify quantity before submitting orders:

```
    buyQuantity, sellQuantity := s.positionRiskControl.ModifiedQuantity(s.Position.Base)
```

It calculates buy and sell quantity shrinking by hard limit and position.

### 3. Circuit-Break Risk Control

Initialization
```
    s.circuitBreakRiskControl = riskcontrol.NewCircuitBreakRiskControl(
        s.Position,
        session.StandardIndicatorSet(s.Symbol).EWMA(
            types.IntervalWindow{
                Window:   EWMAWindow,
                Interval: types.Interval1m,
            }),
        s.CircuitBreakCondition,
        s.ProfitStats)
```
Should pass in position and profit states. Also need an price EWMA to calculate unrealized profit.


Validate parameters:
```
    if s.CircuitBreakCondition.Float64() > 0 {
        return fmt.Errorf("circuitBreakCondition should be non-positive")
    }
    return nil
```
Circuit break condition should be non-greater than zero.

Check for circuit break before submitting orders:
```
    // Circuit break when accumulated losses are over break condition
    if s.circuitBreakRiskControl.IsHalted() {
        return
    }

    submitOrders, err := s.generateSubmitOrders(ctx)
    if err != nil {
        log.WithError(err).Error("failed to generate submit orders")
        return
    }
    log.Infof("submit orders: %+v", submitOrders)

    if s.DryRun {
        log.Infof("dry run, not submitting orders")
        return
    }
```

Notice that if there are multiple place to submit orders, it is recommended to check in one place in Strategy.Run() and re-use that flag before submitting orders. That can avoid duplicated logs generated from IsHalted().
