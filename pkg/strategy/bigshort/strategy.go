package pivotshort

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/datatype/floats"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "bigshort"

var one = fixedpoint.One

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment
	Symbol      string `json:"symbol"`
	Market      types.Market

	// pivot interval and window
	types.IntervalWindow

	Leverage fixedpoint.Value `json:"leverage"`
	Quantity fixedpoint.Value `json:"quantity"`

	// persistence fields

	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`
	TradeStats  *types.TradeStats  `persistence:"trade_stats"`

	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor

	// StrategyController
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})

	if !bbgo.IsBackTesting {
		session.Subscribe(types.MarketTradeChannel, s.Symbol, types.SubscribeOptions{})
	}

	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	return s.orderExecutor.ClosePosition(ctx, percentage)
}

func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	var instanceID = s.InstanceID()

	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	if s.Leverage.IsZero() {
		// the default leverage is 3x
		s.Leverage = fixedpoint.NewFromInt(3)
	}

	// StrategyController
	s.Status = types.StrategyStatusRunning

	s.OnSuspend(func() {
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
	})

	s.OnEmergencyStop(func() {
		// Cancel active orders
		_ = s.orderExecutor.GracefulCancel(ctx)
		// Close 100% position
		_ = s.ClosePosition(ctx, fixedpoint.One)
	})

	// initial required information
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.BindTradeStats(s.TradeStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(s)
	})
	s.orderExecutor.Bind()

	s.ExitMethods.Bind(session, s.orderExecutor)

	pivotLow := session.StandardIndicatorSet(s.Symbol).PivotLow(s.IntervalWindow)
	_ = pivotLow

	dataStore, ok := session.MarketDataStore(s.Symbol)
	if !ok {
		return fmt.Errorf("%s market data store not found", s.Symbol)
	}

	analyzer := &Analyzer{
		pivotLow:  &indicator.PivotLow{IntervalWindow: s.IntervalWindow},
		pivotHigh: &indicator.PivotHigh{IntervalWindow: s.IntervalWindow},
		vwma:      &indicator.VWMA{IntervalWindow: s.IntervalWindow},
	}

	if kLines, ok := dataStore.KLinesOfInterval(types.Interval1h); ok {
		for _, k := range *kLines {
			analyzer.addKLine(k)
		}
	}

	session.MarketDataStream.OnKLineClosed(types.KLineWith(s.Symbol, s.Interval, func(k types.KLine) {
		analyzer.addKLine(k)
	}))

	bbgo.OnShutdown(func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()

		_, _ = fmt.Fprintln(os.Stderr, s.TradeStats.String())
		_ = s.orderExecutor.GracefulCancel(ctx)
	})
	return nil
}

type Area struct {
	// interval is the level of the time frame
	interval types.Interval

	// upper and lower are the price range of this area
	upper, lower float64

	pivots []float64

	testCount int
	weight    float64

	kLines []types.KLine
}

func (a *Area) String() string {
	return fmt.Sprintf("AREA %s range:%f,%f pivots:%d %+v", a.interval, a.lower, a.upper, len(a.pivots), a.pivots)
}

func (a *Area) InRange(price float64) bool {
	return a.lower <= price && price <= a.upper
}

func (a *Area) IsNear(price float64, distance float64) bool {
	return (a.lower*(1.0-distance)) <= price && price <= (a.upper*(1.0+distance))
}

func (a *Area) AddPivot(price float64) {
	for _, p := range a.pivots {
		if p == price {
			return
		}
	}

	a.pivots = append(a.pivots, price)
	sort.Float64s(a.pivots)
}

func (a *Area) Extend(price float64) {
	if price < a.lower {
		a.lower = price
	} else if price > a.upper {
		a.upper = price
	}
	a.AddPivot(price)
}

type Analyzer struct {
	// interval is the level of the time frame
	interval types.Interval

	areas                     []*Area
	highs                     []float64
	lows                      []float64
	previousLow, previousHigh float64
	lastLow, lastHigh         float64

	pivotLow  *indicator.PivotLow
	pivotHigh *indicator.PivotHigh
	vwma      *indicator.VWMA
}

func (a *Analyzer) addKLine(k types.KLine) {
	a.vwma.PushK(k)
	a.pivotLow.PushK(k)
	a.pivotHigh.PushK(k)

	closePrice := k.Close.Float64()

	if a.pivotLow.Last() != a.lastLow {
		low := a.pivotLow.Last()
		a.previousLow = a.lastLow
		a.lastLow = low
		a.lows = append(a.lows, low)

		// lows := floats.Lower(floats.Tail(a.lows, 10), closePrice)
		lows := floats.Lower(a.lows, closePrice)
		if len(lows) > 0 {
			a.updateAreas(lows, k)
		}
	}

	if a.pivotHigh.Last() != a.lastHigh {
		high := a.pivotHigh.Last()
		a.previousHigh = a.lastHigh
		a.lastHigh = high
		a.highs = append(a.highs, high)

		// highs := floats.Higher(floats.Tail(a.highs, 10), closePrice)
		highs := floats.Higher(a.highs, closePrice)
		if len(highs) > 0 {
			a.updateAreas(highs, k)
		}
	}
}

func (a *Analyzer) sortAreas() {
	sort.Slice(a.areas, func(i, j int) bool {
		return a.areas[i].lower < a.areas[j].lower
	})
}

func (a *Analyzer) updateAreas(pivotPrices []float64, k types.KLine) {
	pivotGroups := groupPivots(pivotPrices, 0.01)
	log.Infof("pivot groups: %+v", pivotGroups)
	for _, groupPrices := range pivotGroups {
		// ignore price group that has only one price
		if len(groupPrices) < 2 {
			continue
		}

		upper := floats.Max(groupPrices)
		lower := floats.Min(groupPrices)
		for _, p := range groupPrices {
			_, ok := a.findAndExtendArea(p)
			if !ok {
				// allocate a new area for this
				log.Debugf("add new area: %f ~ %f for %v", lower, upper, groupPrices)
				a.areas = append(a.areas, &Area{
					interval:  k.Interval,
					upper:     upper,
					lower:     lower,
					pivots:    []float64{lower, upper},
					testCount: 0,
					weight:    0,
					kLines:    nil,
				})
			}
		}
	}
	a.sortAreas()

	for i, area := range a.areas {
		log.Infof("Area #%2d: %s", i, area.String())
	}
}

func (a *Analyzer) findAndExtendArea(price float64) (*Area, bool) {
	for _, area := range a.areas {
		if area.InRange(price) {
			area.AddPivot(price)
			return area, true
		} else if area.IsNear(price, 0.003) {
			area.Extend(price)
			return area, true
		}
	}

	return nil, false
}

func groupPivots(prices []float64, minDistance float64) (groups [][]float64) {
	length := len(prices)
	previousPrice := prices[length-1]

	g := []float64{previousPrice}
	for i := length - 1 - 1; i >= 0; i-- {
		price := prices[i]
		if math.Abs(previousPrice-price)/price < minDistance {
			g = append(g, price)
		} else {
			groups = append(groups, g)
			g = []float64{price}
		}
		previousPrice = price
	}

	if len(g) > 0 {
		groups = append(groups, g)
	}

	return groups
}
