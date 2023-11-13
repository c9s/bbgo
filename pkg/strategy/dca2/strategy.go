package dca2

import (
	"context"
	"fmt"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/sirupsen/logrus"
)

const ID = "dca2"

const orderTag = "dca2"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

type Strategy struct {
	Environment *bbgo.Environment
	Market      types.Market

	Symbol string `json:"symbol"`

	// setting
	Budget           fixedpoint.Value `json:"budget"`
	OrderNum         int64            `json:"orderNum"`
	Margin           fixedpoint.Value `json:"margin"`
	TakeProfitSpread fixedpoint.Value `json:"takeProfitSpread"`
	RoundInterval    types.Duration   `json:"roundInterval"`

	// OrderGroupID is the group ID used for the strategy instance for canceling orders
	OrderGroupID uint32 `json:"orderGroupID"`

	// log
	logger    *logrus.Entry
	LogFields logrus.Fields `json:"logFields"`

	// persistence fields: position and profit
	Position    *types.Position    `persistence:"position"`
	ProfitStats *types.ProfitStats `persistence:"profit_stats"`

	// private field
	session       *bbgo.ExchangeSession
	orderExecutor *bbgo.GeneralOrderExecutor
	book          *types.StreamOrderBook
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) Validate() error {
	if s.OrderNum < 1 {
		return fmt.Errorf("maxOrderNum can not be < 1")
	}

	if s.TakeProfitSpread.Compare(fixedpoint.Zero) <= 0 {
		return fmt.Errorf("takeProfitSpread can not be <= 0")
	}

	if s.Margin.Compare(fixedpoint.Zero) <= 0 {
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
	session.Subscribe(types.BookChannel, s.Symbol, types.SubscribeOptions{Depth: types.DepthLevel1})
}

func (s *Strategy) Run(ctx context.Context, _ bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	instanceID := s.InstanceID()
	s.session = session
	s.orderExecutor = bbgo.NewGeneralOrderExecutor(session, s.Symbol, ID, instanceID, s.Position)
	s.orderExecutor.BindEnvironment(s.Environment)
	s.orderExecutor.BindProfitStats(s.ProfitStats)
	s.orderExecutor.TradeCollector().OnPositionUpdate(func(position *types.Position) {
		bbgo.Sync(ctx, s)
	})
	s.orderExecutor.Bind()
	s.book = types.NewStreamBook(s.Symbol)
	s.book.BindStream(s.session.MarketDataStream)

	balances := session.GetAccount().Balances()
	balance := balances[s.Market.QuoteCurrency]
	if balance.Available.Compare(s.Budget) < 0 {
		return fmt.Errorf("the available balance of %s is %s which is less than budget setting %s, please check it", s.Market.QuoteCurrency, balance.Available, s.Budget)
	}

	session.MarketDataStream.OnBookUpdate(func(book types.SliceOrderBook) {
		bid, ok := book.BestBid()
		if !ok {
			return
		}

		takeProfitPrice := s.Market.TruncatePrice(s.Position.AverageCost.Mul(fixedpoint.One.Add(s.TakeProfitSpread)))
		if bid.Price.Compare(takeProfitPrice) >= 0 {
		}
	})

	return nil
}

func (s *Strategy) generateMakerOrder(budget, askPrice, margin fixedpoint.Value, orderNum int64) ([]types.SubmitOrder, error) {
	marginPrice := askPrice.Mul(margin)
	price := askPrice
	var prices []fixedpoint.Value
	var total fixedpoint.Value
	for i := 0; i < int(orderNum); i++ {
		price = price.Sub(marginPrice)
		truncatePrice := s.Market.TruncatePrice(price)
		prices = append(prices, truncatePrice)
		total = total.Add(truncatePrice)
	}

	quantity := budget.Div(total)
	quantity = s.Market.TruncateQuantity(quantity)

	var submitOrders []types.SubmitOrder

	for _, price := range prices {
		submitOrders = append(submitOrders, types.SubmitOrder{
			Symbol:      s.Symbol,
			Market:      s.Market,
			Type:        types.OrderTypeLimit,
			Price:       price,
			Side:        types.SideTypeBuy,
			TimeInForce: types.TimeInForceGTC,
			Quantity:    quantity,
			Tag:         orderTag,
			GroupID:     s.OrderGroupID,
		})
	}

	return submitOrders, nil
}

func (s *Strategy) generateTakeProfitOrder(position *types.Position, takeProfitSpread fixedpoint.Value) types.SubmitOrder {
	takeProfitPrice := s.Market.TruncatePrice(position.AverageCost.Mul(fixedpoint.One.Add(takeProfitSpread)))
	return types.SubmitOrder{
		Symbol:      s.Symbol,
		Market:      s.Market,
		Type:        types.OrderTypeLimit,
		Price:       takeProfitPrice,
		Side:        types.SideTypeSell,
		TimeInForce: types.TimeInForceGTC,
		Quantity:    position.GetBase().Abs(),
		Tag:         orderTag,
		GroupID:     s.OrderGroupID,
	}
}
