package dca2

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
	"github.com/sirupsen/logrus"
)

const ID = "dca2"

const orderTag = "dca2"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	common.Strategy

	Environment *bbgo.Environment
	Market      types.Market

	Symbol string `json:"symbol"`

	// setting
	Short            bool             `json:"short"`
	Budget           fixedpoint.Value `json:"budget"`
	MaxOrderNum      int64            `json:"maxOrderNum"`
	PriceDeviation   fixedpoint.Value `json:"priceDeviation"`
	TakeProfitRatio  fixedpoint.Value `json:"takeProfitRatio"`
	CoolDownInterval types.Duration   `json:"coolDownInterval"`

	// OrderGroupID is the group ID used for the strategy instance for canceling orders
	OrderGroupID uint32 `json:"orderGroupID"`

	// log
	logger    *logrus.Entry
	LogFields logrus.Fields `json:"logFields"`

	// private field
	mu                   sync.Mutex
	openPositionSide     types.SideType
	takeProfitSide       types.SideType
	takeProfitPrice      fixedpoint.Value
	startTimeOfNextRound time.Time
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.MaxOrderNum < 1 {
		return fmt.Errorf("maxOrderNum can not be < 1")
	}

	if s.TakeProfitRatio.Sign() <= 0 {
		return fmt.Errorf("takeProfitSpread can not be <= 0")
	}

	if s.PriceDeviation.Sign() <= 0 {
		return fmt.Errorf("margin can not be <= 0")
	}

	// TODO: validate balance is enough
	return nil
}

func (s *Strategy) Defaults() error {
	if s.LogFields == nil {
		s.LogFields = logrus.Fields{}
	}

	s.LogFields["symbol"] = s.Symbol
	s.LogFields["strategy"] = ID
	return nil
}

func (s *Strategy) Initialize() error {
	s.logger = log.WithFields(s.LogFields)
	return nil
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s-%s", ID, s.Symbol)
}

func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: types.Interval1m})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, s.InstanceID())
	instanceID := s.InstanceID()

	if s.Short {
		s.openPositionSide = types.SideTypeSell
		s.takeProfitSide = types.SideTypeBuy
	} else {
		s.openPositionSide = types.SideTypeBuy
		s.takeProfitSide = types.SideTypeSell
	}

	if s.OrderGroupID == 0 {
		s.OrderGroupID = util.FNV32(instanceID) % math.MaxInt32
	}

	// order executor
	s.OrderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		s.logger.Infof("position: %s", s.Position.String())
		bbgo.Sync(ctx, s)

		// update take profit price here
	})

	session.MarketDataStream.OnKLine(func(kline types.KLine) {
		// check price here
	})

	session.UserDataStream.OnAuth(func() {
		s.logger.Info("user data stream authenticated, start the process")
		// decide state here
	})

	balances, err := session.Exchange.QueryAccountBalances(ctx)
	if err != nil {
		return err
	}

	balance := balances[s.Market.QuoteCurrency]
	if balance.Available.Compare(s.Budget) < 0 {
		return fmt.Errorf("the available balance of %s is %s which is less than budget setting %s, please check it", s.Market.QuoteCurrency, balance.Available, s.Budget)
	}

	return nil
}
