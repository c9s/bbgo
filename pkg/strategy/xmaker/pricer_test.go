package xmaker

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
			name: "buy, i=0, no margin",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(10.0),
			},
			calls: calls{
				i:     0,
				price: Number(100.0),
				rst:   Number(100.0),
			},
		},
		{
			name: "buy, i=1, margin=10",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(10.0),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(110.0),
			},
		},
		{
			name: "sell, i=1, margin=10",
			args: args{
				side:   types.SideTypeSell,
				margin: Number(10.0),
			},
			calls: calls{
				i:     1,
				price: Number(100.0),
				rst:   Number(90.0),
			},
		},
		{
			name: "buy, i=2, margin=5",
			args: args{
				side:   types.SideTypeBuy,
				margin: Number(5.0),
			},
			calls: calls{
				i:     2,
				price: Number(50.0),
				rst:   Number(55.0),
			},
		},
		{
			name: "sell, i=2, margin=5",
			args: args{
				side:   types.SideTypeSell,
				margin: Number(5.0),
			},
			calls: calls{
				i:     2,
				price: Number(50.0),
				rst:   Number(45.0),
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
			pricer := FromDepth(tt.side, tt.book, tt.depth)
			got := pricer(tt.call.i, tt.call.price)
			assert.Equal(t, tt.call.want, got)
		})
	}
}
