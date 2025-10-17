package xvs

import (
	"context"
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/bbgo"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/indicator"
	"github.com/c9s/bbgo/pkg/strategy/common"
	"github.com/c9s/bbgo/pkg/types"
)

const ID = "xvs"

var log = logrus.WithField("strategy", ID)

func init() {
	bbgo.RegisterStrategy(ID, &Strategy{})
}

// Strategy XVS 策略主結構
// 基於大成交量突破的進場信號和吞噬線型態的出場信號
type Strategy struct {
	*common.Strategy

	Market      types.Market
	Environment *bbgo.Environment

	Symbol            string                 `json:"symbol"`
	Quantity          fixedpoint.Value       `json:"quantity"`
	ScaleQuantity     *bbgo.PriceVolumeScale `json:"scaleQuantity"`
	MaxExposure       fixedpoint.Value       `json:"maxExposure"`
	PriceRatioProtect fixedpoint.Value       `json:"priceRatioProtect"` // 價格保護比例 (例如: 1.05 表示 5% 滑點)

	// 進場條件參數
	VolumeInterval           types.Interval   `json:"volumeInterval"`           // 監控成交量的時間間隔 (e.g. 5m 或 15m)
	VolumeThreshold          fixedpoint.Value `json:"volumeThreshold"`          // 基礎資產成交量門檻 (e.g. >= 800 BTC)
	VolumeThresholdInQuote   fixedpoint.Value `json:"volumeThresholdInQuote"`   // 計價資產成交量門檻 (e.g. > 40M USDT)
	MinKLineLowerShadowRatio fixedpoint.Value `json:"minKLineLowerShadowRatio"` // 最小影線比例 (0.1 表示 90% 實體)

	// EMA 技術指標設定
	LongTermEMAWindow  types.IntervalWindow `json:"longTermEMAWindow"`  // 長期EMA視窗設定，可自訂天數與週期
	ShortTermEMAWindow types.IntervalWindow `json:"shortTermEMAWindow"` // 短期EMA視窗設定，可自訂天數與週期

	// Pivot High 設定
	PivotHighWindow types.IntervalWindow `json:"pivotHighWindow"` // Pivot High 視窗設定

	// 出場條件參數
	EngulfingInterval types.Interval `json:"engulfingInterval"` // 監控吞噬型態的時間間隔 (30m)

	// 出場方法集合 (支援停損等多種出場方式)
	ExitMethods bbgo.ExitMethodSet `json:"exits"`

	// 核心組件
	// orderExecutor *bbgo.GeneralOrderExecutor // 訂單執行器

	// 技術指標
	pivotHigh    *indicator.PivotHigh // 軸心高點指標
	longTermEMA  *indicator.EWMA      // 長期EMA指數移動平均線
	shortTermEMA *indicator.EWMA      // 短期EMA指數移動平均線

	// 吞噬型態追踪
	lastGreenKline *types.KLine // 記錄最後一根綠色K線，用於吞噬型態判斷

	// 持久化數據
	TradeStats *types.TradeStats `persistence:"trade_stats"` // 交易統計

	// 策略控制器 (支援暫停、緊急停止等控制功能)
	bbgo.StrategyController
}

func (s *Strategy) ID() string {
	return ID
}

func (s *Strategy) InstanceID() string {
	return fmt.Sprintf("%s:%s", ID, s.Symbol)
}

// Validate 驗證策略參數的有效性
func (s *Strategy) Validate() error {
	if s.Quantity.IsZero() && s.ScaleQuantity == nil {
		return fmt.Errorf("quantity or scaleQuantity can not be zero")
	}
	// 至少要設定一種成交量門檻
	if s.VolumeThreshold.IsZero() && s.VolumeThresholdInQuote.IsZero() {
		return fmt.Errorf("either volumeThreshold or volumeThresholdInQuote must be set")
	}
	// 必須設定最大持倉暴露額度
	if s.MaxExposure.IsZero() {
		return fmt.Errorf("maxExposure must be positive")
	}
	return nil
}

// Defaults 設定策略參數的預設值
func (s *Strategy) Defaults() error {
	// 預設監控 5 分鐘 K線的成交量
	if s.VolumeInterval == "" {
		s.VolumeInterval = types.Interval5m
	}

	// 預設監控 30 分鐘 K線的吞噬型態
	if s.EngulfingInterval == "" {
		s.EngulfingInterval = types.Interval30m
	}

	// 預設最小下影線比例為 50% (即實體佔 50%)
	if s.MinKLineLowerShadowRatio.IsZero() {
		s.MinKLineLowerShadowRatio = fixedpoint.NewFromFloat(0.5)
	}

	// 預設使用 120 倍 volume interval for long term EMA
	if s.LongTermEMAWindow.Interval == "" {
		s.LongTermEMAWindow.Interval = s.VolumeInterval
	}

	if s.LongTermEMAWindow.Window == 0 {
		s.LongTermEMAWindow.Window = 120
	}

	// 預設使用 20 倍 volume interval for short term EMA
	if s.ShortTermEMAWindow.Interval == "" {
		s.ShortTermEMAWindow.Interval = s.VolumeInterval
	}

	if s.ShortTermEMAWindow.Window == 0 {
		s.ShortTermEMAWindow.Window = 20
	}

	// 預設 Pivot High 設定
	if s.PivotHighWindow.Interval == "" {
		s.PivotHighWindow.Interval = s.VolumeInterval
	}

	if s.PivotHighWindow.Window == 0 {
		s.PivotHighWindow.Window = 5 // 預設左右各檢查 5 個點
	}

	return nil
}

func (s *Strategy) Initialize() error {
	if s.Strategy == nil {
		s.Strategy = &common.Strategy{}
	}

	return nil
}

// Subscribe 訂閱必要的市場數據
func (s *Strategy) Subscribe(session *bbgo.ExchangeSession) {
	// 訂閱成交量監控間隔的K線數據 (用於進場信號)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.VolumeInterval})

	// 訂閱吞噬型態監控間隔的K線數據 (用於出場信號)
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.EngulfingInterval})

	// 訂閱EMA指標所需的K線數據
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.LongTermEMAWindow.Interval})
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.ShortTermEMAWindow.Interval})

	// 訂閱Pivot High指標所需的K線數據
	session.Subscribe(types.KLineChannel, s.Symbol, types.SubscribeOptions{Interval: s.PivotHighWindow.Interval})

	// 訂閱出場方法所需的數據
	s.ExitMethods.SetAndSubscribe(session, s)
}

func (s *Strategy) CurrentPosition() *types.Position {
	return s.Position
}

// ClosePosition 平倉指定百分比的持倉
func (s *Strategy) ClosePosition(ctx context.Context, percentage fixedpoint.Value) error {
	base := s.Position.GetBase()
	if base.IsZero() {
		return fmt.Errorf("no opened %s position", s.Position.Symbol)
	}

	// 計算平倉數量
	quantity := base.Mul(percentage).Abs()
	side := types.SideTypeBuy
	if base.Sign() > 0 {
		side = types.SideTypeSell
	}

	// 檢查最小數量要求
	if quantity.Compare(s.Market.MinQuantity) < 0 {
		return fmt.Errorf("order quantity %v is too small, less than %v", quantity, s.Market.MinQuantity)
	}

	submitOrder := types.SubmitOrder{
		Symbol:   s.Symbol,
		Side:     side,
		Type:     types.OrderTypeMarket,
		Quantity: quantity,
		Market:   s.Market,
	}

	bbgo.Notify("Submitting %s %s order to close position by %v", s.Symbol, side.String(), percentage, submitOrder)
	_, err := s.OrderExecutor.SubmitOrders(ctx, submitOrder)
	return err
}

// checkVolumeBreakout 檢查是否出現大成交量突破
func (s *Strategy) checkVolumeBreakout(kline types.KLine) bool {
	// 檢查基礎資產成交量門檻 (例如: >= 800 BTC)
	if !s.VolumeThreshold.IsZero() && kline.Volume.Compare(s.VolumeThreshold) < 0 {
		return false
	}

	// 檢查計價資產成交量門檻 (例如: > 40M USDT)
	if !s.VolumeThresholdInQuote.IsZero() && kline.QuoteVolume.Compare(s.VolumeThresholdInQuote) < 0 {
		return false
	}

	// 檢查K線的下影線比例
	totalSize := kline.GetHigh().Sub(kline.GetLow()) // K線總長度
	if totalSize.IsZero() {
		return false
	}

	lowerShadowSize := fixedpoint.Min(kline.GetOpen(), kline.GetClose()).Sub(kline.GetLow()) // K線下影線長度
	lowerShadowRatio := lowerShadowSize.Div(totalSize)
	if lowerShadowRatio.Compare(s.MinKLineLowerShadowRatio) < 0 {
		log.Infof("Kline has too small lower shadow ratio %s, required >= %s", lowerShadowRatio, s.MinKLineLowerShadowRatio)
		return false
	}

	return true
}

// checkEMAFilter 檢查EMA趨勢過濾條件
func (s *Strategy) checkEMAFilter(closePrice fixedpoint.Value) bool {
	if s.longTermEMA == nil || s.shortTermEMA == nil {
		return false
	}

	longTermEMAValue := fixedpoint.NewFromFloat(s.longTermEMA.Last(0))
	shortTermEMAValue := fixedpoint.NewFromFloat(s.shortTermEMA.Last(0))
	// 如果價格從長期EMA上方跌破到下方，跳過進場
	if closePrice.Compare(longTermEMAValue) < 0 {
		log.Infof("Price %s is below long term EMA %s, skipping entry", closePrice, longTermEMAValue)
		return false
	}

	if closePrice.Compare(shortTermEMAValue) > 0 {
		log.Infof("Price %s is above short term EMA %s, skipping entry", closePrice, shortTermEMAValue)
		return false
	}

	return true
}

// checkEngulfingExit 檢查吞噬型態出場條件
func (s *Strategy) checkEngulfingExit(kline types.KLine) bool {
	if s.lastGreenKline == nil {
		return false
	}
	log.Infof("[Exit] Checking engulfing exit for kline %s, last green kline %s", kline.String(), s.lastGreenKline.String())

	// 當前K線必須是紅色且收盤價低於前一根綠色K線的開盤價
	if kline.GetClose().Compare(kline.GetOpen()) >= 0 {
		return false // 不是紅色K線
	}

	if kline.GetClose().Compare(s.lastGreenKline.GetOpen()) >= 0 {
		return false // 沒有低於前一根綠色K線的開盤價
	}

	// 成交量必須大於前一根綠色K線
	if kline.Volume.Compare(s.lastGreenKline.Volume) <= 0 {
		return false
	}

	return true
}

func (s *Strategy) setLastGreenKLine(kline *types.KLine) {
	if kline.GetClose().Compare(kline.GetOpen()) < 0 {
		s.lastGreenKline = kline
	}
}

// Run 策略主要運行邏輯
func (s *Strategy) Run(ctx context.Context, orderExecutor bbgo.OrderExecutor, session *bbgo.ExchangeSession) error {
	instanceID := s.InstanceID()

	// 初始化持久化數據結構
	if s.Position == nil {
		s.Position = types.NewPositionFromMarket(s.Market)
	}

	if s.ProfitStats == nil {
		s.ProfitStats = types.NewProfitStats(s.Market)
	}

	if s.TradeStats == nil {
		s.TradeStats = types.NewTradeStats(s.Symbol)
	}

	// initialize common strategy
	s.Strategy.Initialize(ctx, s.Environment, session, s.Market, ID, instanceID)
	s.OrderExecutor.BindTradeStats(s.TradeStats)

	// 初始化技術指標
	standardIndicatorSet := session.StandardIndicatorSet(s.Symbol)
	s.longTermEMA = standardIndicatorSet.EWMA(s.LongTermEMAWindow)
	s.shortTermEMA = standardIndicatorSet.EWMA(s.ShortTermEMAWindow)
	s.pivotHigh = standardIndicatorSet.PivotHigh(s.PivotHighWindow)

	// 設定出場方法
	s.ExitMethods.Bind(session, s.OrderExecutor)

	// 策略控制器初始化
	s.Status = types.StrategyStatusRunning

	// 暫停策略時的處理
	s.OnSuspend(func() {
		// 取消所有訂單
		_ = s.OrderExecutor.GracefulCancel(ctx)
		bbgo.Sync(ctx, s)
	})

	// 緊急停止策略時的處理
	/*
		s.OnEmergencyStop(func() {
			// 平倉100%持倉
			percentage := fixedpoint.NewFromFloat(1.0)
			if err := s.ClosePosition(context.Background(), percentage); err != nil {
				errMsg := "failed to close position"
				log.WithError(err).Errorf(errMsg)
				bbgo.Notify(errMsg)
			}

			if err := s.Suspend(); err != nil {
				errMsg := "failed to suspend strategy"
				log.WithError(err).Errorf(errMsg)
				bbgo.Notify(errMsg)
			}
		})
	*/

	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// log.Info(kline.String())
	})

	// 監聽K線收盤事件 - 進場
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		// 策略未運行
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// K線不是目標交易對
		if kline.Symbol != s.Symbol {
			return
		}

		// 不是進場用的 Interval
		if kline.Interval != s.VolumeInterval {
			return
		}

		// 已達最大持倉暴露額度
		log.Infof("[Entry] Current position base %s, max exposure %s", s.Position.GetBase(), s.MaxExposure)
		if s.Position.GetBase().Compare(s.MaxExposure) >= 0 {
			return
		}

		// 檢查成交量突破條件以及是否收線
		log.Infof("[Entry] Checking volume breakout for kline %s", kline.String())
		if !s.checkVolumeBreakout(kline) {
			return
		}

		closePrice := kline.GetClose()
		log.Infof("[Entry] Checking if closed price %f is above long term EMA %f and below short term EMA %f", closePrice.Float64(), s.longTermEMA.Last(0), s.shortTermEMA.Last(0))
		if !s.checkEMAFilter(closePrice) {
			return
		}

		bbgo.Notify("Found %s volume breakout: volume %s, quote volume %s, price %s",
			s.Symbol,
			kline.Volume.String(),
			kline.QuoteVolume.String(),
			closePrice.String(),
			kline)

		var quantity fixedpoint.Value
		if !s.Quantity.IsZero() {
			quantity = s.Quantity
		} else {
			if q, err := s.ScaleQuantity.Scale(closePrice.Float64(), kline.Volume.Float64()); err != nil {
				log.WithError(err).Error("scale quantity error")
				return
			} else {
				quantity = fixedpoint.NewFromFloat(q)
			}
		}
		orderForm := types.SubmitOrder{
			Symbol:   s.Symbol,
			Market:   s.Market,
			Side:     types.SideTypeBuy,
			Type:     types.OrderTypeMarket,
			Quantity: quantity,
		}

		if !s.PriceRatioProtect.IsZero() {
			orderForm.Type = types.OrderTypeLimit
			orderForm.Price = closePrice.Mul(fixedpoint.One.Add(s.PriceRatioProtect.Abs())) // 設定保護價格
			orderForm.TimeInForce = types.TimeInForceIOC
		}

		bbgo.Notify("Submitting %s market buy order with quantity %s",
			s.Symbol,
			s.Quantity.String(),
			orderForm)

		if _, err := s.OrderExecutor.SubmitOrders(ctx, orderForm); err != nil {
			log.WithError(err).Error("submit order error")
		}
	})

	// 監聽K線收盤事件 - 停利
	session.MarketDataStream.OnKLineClosed(func(kline types.KLine) {
		if s.Status != types.StrategyStatusRunning {
			return
		}

		// K線不是目標交易對
		if kline.Symbol != s.Symbol {
			return
		}

		// 不是進場用的 Interval
		if kline.Interval != s.EngulfingInterval {
			return
		}

		// 價格低於成本價，跳過停利出場檢查
		if s.Position.GetAverageCost().Compare(kline.GetClose()) >= 0 {
			return
		}

		// 只有當價格超過 pivot high 時才能停利出場
		if s.pivotHigh != nil && s.pivotHigh.Length() > 0 {
			currentPivotHigh := fixedpoint.NewFromFloat(s.pivotHigh.Last(0))
			if kline.GetClose().Compare(currentPivotHigh) <= 0 {
				log.Infof("[Exit] Price %s has not exceeded pivot high %s, skipping exit",
					kline.GetClose(), currentPivotHigh)
				return
			}
			log.Infof("[Exit] Price %s exceeded pivot high %s, allowing exit",
				kline.GetClose(), currentPivotHigh)
		}

		// 設定最後一根綠K線
		defer s.setLastGreenKLine(&kline)

		log.Infof("[Exit] Current position base %s, min quantity %s", s.Position.GetBase(), s.Market.MinQuantity)
		if s.Position.GetBase().Compare(s.Market.MinQuantity) < 0 {
			return
		}

		// 檢查吞噬出場型態
		if !s.checkEngulfingExit(kline) {
			return
		}

		bbgo.Notify("Found %s engulfing exit pattern: red kline %s with volume %s > previous green volume %s",
			s.Symbol,
			kline.GetClose().String(),
			kline.Volume.String(),
			s.lastGreenKline.Volume.String(),
			kline)

		// 平倉100%持倉
		if err := s.ClosePosition(ctx, fixedpoint.One); err != nil {
			log.WithError(err).Error("close position error")
		}
	})

	// 優雅關閉處理
	bbgo.OnShutdown(ctx, func(ctx context.Context, wg *sync.WaitGroup) {
		defer wg.Done()
		_ = s.OrderExecutor.GracefulCancel(ctx)
	})

	return nil
}
