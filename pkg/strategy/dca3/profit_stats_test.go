package dca3

import (
	"testing"
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func createTestMarketForProfitStats() types.Market {
	return types.Market{
		Symbol:        "BTCUSDT",
		BaseCurrency:  "BTC",
		QuoteCurrency: "USDT",
		TickSize:      fixedpoint.NewFromFloat(0.01),
		StepSize:      fixedpoint.NewFromFloat(0.000001),
		MinNotional:   fixedpoint.NewFromFloat(10.0),
		MinQuantity:   fixedpoint.NewFromFloat(0.001),
	}
}

func createTestTrade(side types.SideType, price, quantity, fee, quoteQuantity string, feeCurrency string) types.Trade {
	return types.Trade{
		ID:            123,
		OrderID:       456,
		Symbol:        "BTCUSDT",
		Side:          side,
		Price:         fixedpoint.MustNewFromString(price),
		Quantity:      fixedpoint.MustNewFromString(quantity),
		QuoteQuantity: fixedpoint.MustNewFromString(quoteQuantity),
		Fee:           fixedpoint.MustNewFromString(fee),
		FeeCurrency:   feeCurrency,
		Time:          types.Time(time.Now()),
	}
}

func TestNewProfitStats(t *testing.T) {
	market := createTestMarketForProfitStats()
	quoteInvestment := fixedpoint.NewFromFloat(1000)

	stats := newProfitStats(market, quoteInvestment)

	assert.Equal(t, market.Symbol, stats.Symbol)
	assert.Equal(t, market, stats.Market)
	assert.Equal(t, int64(1), stats.Round)
	assert.Equal(t, quoteInvestment, stats.QuoteInvestment)
	assert.True(t, stats.CurrentRoundProfit.IsZero())
	assert.True(t, stats.TotalProfit.IsZero())
	assert.NotNil(t, stats.CurrentRoundFee)
	assert.NotNil(t, stats.TotalFee)
	assert.Equal(t, 0, len(stats.CurrentRoundFee))
	assert.Equal(t, 0, len(stats.TotalFee))
}

func TestProfitStats_AddTrade_BuyTrade(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 買單：花費 1000 USDT 買入 0.033 BTC，手續費 0.0015 BTC
	buyTrade := createTestTrade(
		types.SideTypeBuy,
		"30000",    // price
		"0.033333", // quantity
		"0.0015",   // fee
		"1000",     // quoteQuantity
		"BTC",      // feeCurrency
	)

	stats.AddTrade(buyTrade)

	// 買單應該減少 CurrentRoundProfit (支出)
	expectedProfit := fixedpoint.NewFromFloat(-1000) // -quoteQuantity
	assert.Equal(t, expectedProfit, stats.CurrentRoundProfit)
	assert.Equal(t, expectedProfit, stats.TotalProfit)

	// 檢查手續費
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.CurrentRoundFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.TotalFee["BTC"])
}

func TestProfitStats_AddTrade_SellTrade(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 賣單：賣出 0.033 BTC 獲得 1050 USDT，手續費 1.05 USDT
	sellTrade := createTestTrade(
		types.SideTypeSell,
		"31818.18", // price
		"0.033",    // quantity
		"1.05",     // fee
		"1050",     // quoteQuantity
		"USDT",     // feeCurrency
	)

	stats.AddTrade(sellTrade)

	// 賣單應該增加 CurrentRoundProfit (收入)，但要扣除手續費
	expectedProfit := fixedpoint.NewFromFloat(1050).Sub(fixedpoint.NewFromFloat(1.05))
	assert.Equal(t, expectedProfit, stats.CurrentRoundProfit)
	assert.Equal(t, expectedProfit, stats.TotalProfit)

	// 檢查手續費
	assert.Equal(t, fixedpoint.NewFromFloat(1.05), stats.CurrentRoundFee["USDT"])
	assert.Equal(t, fixedpoint.NewFromFloat(1.05), stats.TotalFee["USDT"])
}

func TestProfitStats_AddTrade_CompleteTradeCycle(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 完整的交易週期：買入 -> 賣出

	// 1. 買單：花費 1000 USDT
	buyTrade := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.033333",
		"0.0015",
		"1000",
		"BTC",
	)
	stats.AddTrade(buyTrade)

	// 2. 賣單：獲得 1100 USDT，手續費 1.1 USDT
	sellTrade := createTestTrade(
		types.SideTypeSell,
		"33333.33",
		"0.033",
		"1.1",
		"1100",
		"USDT",
	)
	stats.AddTrade(sellTrade)

	// 總獲利應該是：-1000 (買入) + 1100 (賣出) - 1.1 (手續費) = 98.9
	expectedProfit := fixedpoint.NewFromFloat(98.9)
	assert.Equal(t, expectedProfit, stats.CurrentRoundProfit)
	assert.Equal(t, expectedProfit, stats.TotalProfit)

	// 檢查手續費
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.CurrentRoundFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(1.1), stats.CurrentRoundFee["USDT"])
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.TotalFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(1.1), stats.TotalFee["USDT"])
}

func TestProfitStats_AddTrade_MultipleFeeCurrencies(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 第一筆交易：BTC 手續費
	trade1 := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.033",
		"0.001",
		"990",
		"BTC",
	)
	stats.AddTrade(trade1)

	// 第二筆交易：同樣是 BTC 手續費，應該累加
	trade2 := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.001",
		"0.0005",
		"30",
		"BTC",
	)
	stats.AddTrade(trade2)

	// 第三筆交易：USDT 手續費
	trade3 := createTestTrade(
		types.SideTypeSell,
		"31000",
		"0.034",
		"1.054",
		"1054",
		"USDT",
	)
	stats.AddTrade(trade3)

	// 檢查手續費累計
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.CurrentRoundFee["BTC"]) // 0.001 + 0.0005
	assert.Equal(t, fixedpoint.NewFromFloat(1.054), stats.CurrentRoundFee["USDT"])
	assert.Equal(t, fixedpoint.NewFromFloat(0.0015), stats.TotalFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(1.054), stats.TotalFee["USDT"])
}

func TestProfitStats_AddTrade_QuoteCurrencyFeeDeduction(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 當手續費是計價貨幣(USDT)時，應該從獲利中扣除
	sellTrade := createTestTrade(
		types.SideTypeSell,
		"31000",
		"0.033",
		"1.023", // USDT 手續費
		"1023",
		"USDT", // 手續費貨幣是計價貨幣
	)
	stats.AddTrade(sellTrade)

	// 獲利計算：1023 (收入) - 1.023 (手續費) = 1021.977
	expectedProfit := fixedpoint.NewFromFloat(1021.977)
	assert.Equal(t, expectedProfit, stats.CurrentRoundProfit)
	assert.Equal(t, expectedProfit, stats.TotalProfit)
}

func TestProfitStats_AddTrade_NonQuoteCurrencyFeeNoDeduction(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 當手續費不是計價貨幣時，不從獲利中扣除
	buyTrade := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.033",
		"0.0015", // BTC 手續費
		"990",
		"BTC", // 手續費貨幣不是計價貨幣
	)
	stats.AddTrade(buyTrade)

	// 獲利計算：-990 (支出)，手續費不影響 USDT 獲利計算
	expectedProfit := fixedpoint.NewFromFloat(-990)
	assert.Equal(t, expectedProfit, stats.CurrentRoundProfit)
	assert.Equal(t, expectedProfit, stats.TotalProfit)
}

func TestProfitStats_NewRound(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 第一輪添加一些交易
	buyTrade := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.01",
		"0.0005",
		"300",
		"BTC",
	)
	stats.AddTrade(buyTrade)

	sellTrade := createTestTrade(
		types.SideTypeSell,
		"35000",
		"0.0095",
		"16.625",
		"332.5",
		"USDT",
	)
	stats.AddTrade(sellTrade)

	// 記錄第一輪的數據
	// firstRoundProfit := stats.CurrentRoundProfit
	firstRoundTotalProfit := stats.TotalProfit

	// 開始新一輪
	stats.NewRound()

	// 檢查輪次增加
	assert.Equal(t, int64(2), stats.Round)

	// 檢查當前輪次獲利重置
	assert.True(t, stats.CurrentRoundProfit.IsZero())

	// 檢查總獲利保持不變
	assert.Equal(t, firstRoundTotalProfit, stats.TotalProfit)

	// 檢查當前輪次手續費重置
	assert.Equal(t, 0, len(stats.CurrentRoundFee))

	// 檢查總手續費保持不變
	assert.Equal(t, fixedpoint.NewFromFloat(0.0005), stats.TotalFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(16.625), stats.TotalFee["USDT"])

	// 第二輪添加新交易
	newTrade := createTestTrade(
		types.SideTypeBuy,
		"32000",
		"0.01",
		"0.0005",
		"320",
		"BTC",
	)
	stats.AddTrade(newTrade)

	// 檢查當前輪次獲利
	assert.Equal(t, fixedpoint.NewFromFloat(-320), stats.CurrentRoundProfit)

	// 檢查總獲利更新
	expectedTotalProfit := firstRoundTotalProfit.Add(fixedpoint.NewFromFloat(-320))
	assert.Equal(t, expectedTotalProfit, stats.TotalProfit)

	// 檢查手續費累計
	assert.Equal(t, fixedpoint.NewFromFloat(0.0005), stats.CurrentRoundFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(0.001), stats.TotalFee["BTC"]) // 0.0005 + 0.0005
}

func TestProfitStats_String(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))
	stats.FromOrderID = 12345

	// 添加一些交易數據
	stats.AddTrade(createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.033",
		"0.001",
		"990",
		"BTC",
	))

	stats.AddTrade(createTestTrade(
		types.SideTypeSell,
		"31000",
		"0.033",
		"1.023",
		"1023",
		"USDT",
	))

	result := stats.String()

	// 檢查字符串包含關鍵信息
	assert.Contains(t, result, "Profit Stats")
	assert.Contains(t, result, "Round: 1")
	assert.Contains(t, result, "From Order ID: 12345")
	assert.Contains(t, result, "Quote Investment: 1000")
	assert.Contains(t, result, "Current Round Profit:")
	assert.Contains(t, result, "Total Profit:")
	assert.Contains(t, result, "FEE (BTC): 0.001")
	assert.Contains(t, result, "FEE (USDT): 1.023")
}

func TestProfitStats_InitializationState(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(2000))

	// 測試初始狀態
	assert.Equal(t, "BTCUSDT", stats.Symbol)
	assert.Equal(t, market, stats.Market)
	assert.Equal(t, int64(1), stats.Round)
	assert.Equal(t, uint64(0), stats.FromOrderID)
	assert.Equal(t, fixedpoint.NewFromFloat(2000), stats.QuoteInvestment)
	assert.True(t, stats.CurrentRoundProfit.IsZero())
	assert.True(t, stats.TotalProfit.IsZero())
	assert.Empty(t, stats.CurrentRoundFee)
	assert.Empty(t, stats.TotalFee)
	assert.Empty(t, stats.OpenPositionPVs)
}

func TestProfitStats_AddTrade_NilMaps(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := &ProfitStats{
		Symbol:          market.Symbol,
		Market:          market,
		Round:           1,
		QuoteInvestment: fixedpoint.NewFromFloat(1000),
		// 故意不初始化 maps
	}

	trade := createTestTrade(
		types.SideTypeBuy,
		"30000",
		"0.033",
		"0.001",
		"990",
		"BTC",
	)

	// 應該不會 panic，會自動創建 map
	stats.AddTrade(trade)

	assert.NotNil(t, stats.CurrentRoundFee)
	assert.NotNil(t, stats.TotalFee)
	assert.Equal(t, fixedpoint.NewFromFloat(0.001), stats.CurrentRoundFee["BTC"])
	assert.Equal(t, fixedpoint.NewFromFloat(0.001), stats.TotalFee["BTC"])
}

func TestProfitStats_MultipleRounds(t *testing.T) {
	market := createTestMarketForProfitStats()
	stats := newProfitStats(market, fixedpoint.NewFromFloat(1000))

	// 第一輪：獲利 100
	stats.AddTrade(createTestTrade(types.SideTypeBuy, "30000", "0.033", "0.001", "990", "BTC"))
	stats.AddTrade(createTestTrade(types.SideTypeSell, "33333", "0.033", "1.1", "1100", "USDT"))

	firstRoundProfit := stats.CurrentRoundProfit
	assert.True(t, firstRoundProfit.Compare(fixedpoint.NewFromFloat(98)) > 0) // 約 98.9

	stats.NewRound()

	// 第二輪：獲利 50
	stats.AddTrade(createTestTrade(types.SideTypeBuy, "31000", "0.032", "0.0008", "992", "BTC"))
	stats.AddTrade(createTestTrade(types.SideTypeSell, "32500", "0.032", "1.04", "1040", "USDT"))

	secondRoundProfit := stats.CurrentRoundProfit
	assert.True(t, secondRoundProfit.Compare(fixedpoint.NewFromFloat(46)) > 0) // 約 46.96

	// 檢查總獲利是兩輪的總和
	expectedTotalProfit := firstRoundProfit.Add(secondRoundProfit)
	assert.Equal(t, expectedTotalProfit, stats.TotalProfit)

	// 檢查輪次
	assert.Equal(t, int64(2), stats.Round)

	// 檢查總手續費累計
	assert.Equal(t, fixedpoint.NewFromFloat(0.0018), stats.TotalFee["BTC"]) // 0.001 + 0.0008
	assert.Equal(t, fixedpoint.NewFromFloat(2.14), stats.TotalFee["USDT"])  // 1.1 + 1.04
}
