package pricer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestApplyFeeRate(t *testing.T) {
	type args struct {
		side       types.SideType
		feeRate    fixedpoint.Value
		inputPrice fixedpoint.Value
	}

	type calls struct {
		i     int
		price fixedpoint.Value
		rst   fixedpoint.Value
	}

	tests := []struct {
		name  string
		args  args
		calls calls
	}{
		{
			name: "buy fee rate",
			args: args{
				side:    types.SideTypeBuy,
				feeRate: Number(0.001),
			},
			calls: calls{
				i:     0,
				price: Number(104000.0),
				rst:   Number(104000.0 * (1 - 0.001)),
			},
		},
		{
			name: "sell fee rate",
			args: args{
				side:    types.SideTypeSell,
				feeRate: Number(0.001),
			},
			calls: calls{
				i:     0,
				price: Number(104000.0),
				rst:   Number(104000.0 * (1 + 0.001)),
			},
		},
		{
			name: "buy zero fee rate",
			args: args{
				side:    types.SideTypeBuy,
				feeRate: Number(0.0),
			},
			calls: calls{
				i:     0,
				price: Number(10000.0),
				rst:   Number(10000.0),
			},
		},
		{
			name: "sell zero fee rate",
			args: args{
				side:    types.SideTypeSell,
				feeRate: Number(0.0),
			},
			calls: calls{
				i:     0,
				price: Number(10000.0),
				rst:   Number(10000.0),
			},
		},
		{
			name: "buy negative fee rate (rebate)",
			args: args{
				side:    types.SideTypeBuy,
				feeRate: Number(-0.0005),
			},
			calls: calls{
				i:     0,
				price: Number(20000.0),
				rst:   Number(20000.0 * (1 + 0.0005)),
			},
		},
		{
			name: "sell negative fee rate (rebate)",
			args: args{
				side:    types.SideTypeSell,
				feeRate: Number(-0.0005),
			},
			calls: calls{
				i:     0,
				price: Number(20000.0),
				rst:   Number(20000.0 * (1 - 0.0005)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDeltaf(t, tt.calls.rst.Float64(),
				ApplyFeeRate(tt.args.side, tt.args.feeRate)(tt.calls.i, tt.calls.price).Float64(),
				0.00001, "ApplyFeeRate(%v, %v)", tt.args.side, tt.args.feeRate)
		})
	}
}

func TestAdjustByTick(t *testing.T) {
	type args struct {
		side     types.SideType
		pips     fixedpoint.Value
		tickSize fixedpoint.Value
	}
	type calls struct {
		i     int
		price fixedpoint.Value
		rst   fixedpoint.Value
	}
	tests := []struct {
		name  string
		args  args
		calls calls
	}{
		{
			name: "buy, i=0, no adjust",
			args: args{
				side:     types.SideTypeBuy,
				pips:     Number(1),
				tickSize: Number(0.1),
			},
			calls: calls{
				i:     0,
				price: Number(100.0),
				rst:   Number(100.0),
			},
		},
		{
			name: "buy, i=1, pips=1, tickSize=0.1",
			args: args{
				side:     types.SideTypeBuy,
				pips:     Number(1),
				tickSize: Number(0.1),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(100.1),
			},
		},
		{
			name: "buy, i=2, pips=2, tickSize=0.1",
			args: args{
				side:     types.SideTypeBuy,
				pips:     Number(2),
				tickSize: Number(0.1),
			},
			calls: calls{
				i:     2,
				price: Number(100.0),
				rst:   Number(100.2),
			},
		},
		{
			name: "sell, i=1, pips=1, tickSize=0.1",
			args: args{
				side:     types.SideTypeSell,
				pips:     Number(1),
				tickSize: Number(0.1),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(99.9),
			},
		},
		{
			name: "sell, i=2, pips=2, tickSize=0.1",
			args: args{
				side:     types.SideTypeSell,
				pips:     Number(2),
				tickSize: Number(0.1),
			},
			calls: calls{
				i:     2,
				price: Number(100.0),
				rst:   Number(99.8),
			},
		},
		{
			name: "tickSize=0, no adjust",
			args: args{
				side:     types.SideTypeBuy,
				pips:     Number(1),
				tickSize: Number(0.0),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(100.0),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDeltaf(t, tt.calls.rst.Float64(),
				AdjustByTick(tt.args.side, tt.args.pips, tt.args.tickSize)(tt.calls.i, tt.calls.price).Float64(),
				0.00001, "AdjustByTick(%v, %v, %v)", tt.args.side, tt.args.pips, tt.args.tickSize)
		})
	}
}

func TestApplyMargin(t *testing.T) {
	type args struct {
		side   types.SideType
		margin fixedpoint.Value
	}
	type calls struct {
		i     int
		price fixedpoint.Value
		rst   fixedpoint.Value
	}
	tests := []struct {
		name  string
		args  args
		calls calls
	}{
		{
			name: "buy, i=0, margin=10",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(0.1),
			},
			calls: calls{
				i:     0,
				price: Number(100.0),
				rst:   Number(90.0), // buy: 100 * 0.9
			},
		},
		{
			name: "buy, i=1, margin=10",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(0.1),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(90.0), // buy: 100 * 0.9
			},
		},
		{
			name: "sell, i=1, margin=10",
			args: args{
				side:   types.SideTypeSell,
				margin: Number(0.1),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(110.0), // sell: 100 * 1.1
			},
		},
		{
			name: "buy, i=2, margin=5",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(0.05),
			},
			calls: calls{
				i:     2,
				price: Number(50.0),
				rst:   Number(47.5), // buy: 50 * 0.95
			},
		},
		{
			name: "sell, i=2, margin=5",
			args: args{
				side:   types.SideTypeSell,
				margin: Number(0.05),
			},
			calls: calls{
				i:     2,
				price: Number(50.0),
				rst:   Number(52.5), // sell: 50 * 1.05
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDeltaf(t, tt.calls.rst.Float64(),
				ApplyMargin(tt.args.side, tt.args.margin)(tt.calls.i, tt.calls.price).Float64(),
				0.00001, "ApplyMargin(%v, %v)", tt.args.side, tt.args.margin)
		})
	}
}

func TestFromDepth(t *testing.T) {
	type call struct {
		i     int
		price fixedpoint.Value
		want  fixedpoint.Value
	}

	// 使用 PriceVolumeSliceFromText 塞入測試資料
	book := types.NewStreamBook("BTCUSDT", "")

	// 假設 tickSize = 0.1
	bids := PriceVolumeSliceFromText(`
		100.1, 1
		100.0, 2
	`)

	asks := PriceVolumeSliceFromText(`
		101.0, 1
		102.0, 1
	`)

	now := time.Now()
	book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   bids,
		Asks:   asks,
		Time:   now,
	})

	depthBook := types.NewDepthBook(book)

	tests := []struct {
		name  string
		side  types.SideType
		book  *types.DepthBook
		depth fixedpoint.Value
		call  call
	}{
		{
			name:  "bid side, depth 1",
			side:  types.SideTypeBuy,
			book:  depthBook,
			depth: Number(1),
			call:  call{i: 0, price: Number(0), want: Number(100.1)},
		},
		{
			name:  "ask side, depth 2",
			side:  types.SideTypeSell,
			book:  depthBook,
			depth: Number(2),
			call:  call{i: 0, price: Number(0), want: Number(101.5)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricer := FromDepthBook(tt.side, tt.book, tt.depth)
			got := pricer(tt.call.i, tt.call.price)
			assert.Equal(t, tt.call.want, got)
		})
	}
}

func TestComposePricers(t *testing.T) {
	type call struct {
		i     int
		price fixedpoint.Value
		want  fixedpoint.Value
	}

	tests := []struct {
		name    string
		pricers []Pricer
		call    call
	}{
		{
			name: "apply fee then margin then tick",
			pricers: []Pricer{
				ApplyFeeRate(types.SideTypeBuy, Number(0.001)),          // price * (1 - 0.001)
				ApplyMargin(types.SideTypeBuy, Number(0.1)),             // price * 0.9
				AdjustByTick(types.SideTypeBuy, Number(1), Number(0.1)), // +0.1 if i>0
			},
			call: call{
				i:     1,
				price: Number(100.0),
				want:  Number(90.01), // 100*0.999*0.9+0.1 = 90.01
			},
		},
		{
			name: "apply margin then fee",
			pricers: []Pricer{
				ApplyMargin(types.SideTypeSell, Number(0.05)),   // price * 1.05
				ApplyFeeRate(types.SideTypeSell, Number(0.002)), // *1.002
			},
			call: call{
				i:     2,
				price: Number(200.0),
				want:  Number(210.42), // (200*1.05)*1.002 = 210.42
			},
		},
		{
			name:    "no pricer",
			pricers: []Pricer{},
			call: call{
				i:     0,
				price: Number(123.45),
				want:  Number(123.45),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricer := Compose(tt.pricers...)
			got := pricer(tt.call.i, tt.call.price)
			assert.InDeltaf(t, tt.call.want.Float64(), got.Float64(), 0.00001, "Compose result mismatch")
		})
	}
}

func TestNewCoveredDepth(t *testing.T) {
	book := types.NewStreamBook("BTCUSDT", "")
	bids := PriceVolumeSliceFromText(`
		100.1, 1
		100.0, 2
		99.9,  3
		99.8,  4
	`)
	asks := PriceVolumeSliceFromText(`
		101.0, 1
		102.0, 1
	`)

	now := time.Now()
	book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   bids,
		Asks:   asks,
		Time:   now,
	})

	depthBook := types.NewDepthBook(book)
	initialDepth := Number(1)

	coveredDepth := NewCoveredDepth(depthBook, initialDepth)
	pricer := coveredDepth.Pricer(types.SideTypeBuy)

	// Assume the order quantity per layer is 1
	layerQty := Number(1)

	tests := []struct {
		name string
		i    int
		want fixedpoint.Value
	}{
		{
			name: "layer 0, depth 1",
			i:    0,
			want: Number(0), // according to implementation, initial is 0
		},
		{
			name: "layer 1, depth 2",
			i:    1,
			want: Number(100.1),
		},
		{
			name: "layer 2, depth 3",
			i:    2,
			want: Number(100.05), // (100.1*1 + 100.0*1)/2
		},
		{
			name: "layer 3, depth 4",
			i:    3,
			want: Number(100.03333333333333), // (100.1*1 + 100.0*1 + 99.9*1)/3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.i > 0 {
				coveredDepth.Cover(layerQty)
			}
			got := pricer(tt.i, Number(0))
			assert.InDeltaf(t, tt.want.Float64(), got.Float64(), 0.001, "NewCoveredDepth.Pricer(%d)", tt.i)
		})
	}
}

func TestFromBestPrice(t *testing.T) {
	book := types.NewStreamBook("BTCUSDT", "")

	bids := PriceVolumeSliceFromText(`
		123.45, 1
		122.00, 2
	`)
	asks := PriceVolumeSliceFromText(`
		234.56, 1
		235.00, 2
	`)

	now := time.Now()
	book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   bids,
		Asks:   asks,
		Time:   now,
	})

	t.Run("buy side returns best bid", func(t *testing.T) {
		pricer := FromBestPrice(types.SideTypeBuy, book)
		got := pricer(0, Number(0))
		assert.Equal(t, Number(123.45), got)
	})

	t.Run("sell side returns best ask", func(t *testing.T) {
		pricer := FromBestPrice(types.SideTypeSell, book)
		got := pricer(0, Number(0))
		assert.Equal(t, Number(234.56), got)
	})

	// clear bids/asks
	book.Load(types.SliceOrderBook{
		Symbol: "BTCUSDT",
		Bids:   types.PriceVolumeSlice{},
		Asks:   types.PriceVolumeSlice{},
		Time:   now,
	})

	t.Run("buy side returns zero if no best bid", func(t *testing.T) {
		pricer := FromBestPrice(types.SideTypeBuy, book)
		got := pricer(0, Number(0))
		assert.Equal(t, fixedpoint.Zero, got)
	})

	t.Run("sell side returns zero if no best ask", func(t *testing.T) {
		pricer := FromBestPrice(types.SideTypeSell, book)
		got := pricer(0, Number(0))
		assert.Equal(t, fixedpoint.Zero, got)
	})
}
