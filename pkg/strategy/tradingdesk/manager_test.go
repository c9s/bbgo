package tradingdesk

import (
	"testing"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	. "github.com/c9s/bbgo/pkg/testing/testhelper"
	"github.com/c9s/bbgo/pkg/types"
)

func TestValidateStopLossTakeProfit(t *testing.T) {
	tests := []struct {
		name            string
		side            types.SideType
		currentPrice    fixedpoint.Value
		stopLossPrice   fixedpoint.Value
		takeProfitPrice fixedpoint.Value
		wantOK          bool
	}{
		{
			name:            "long valid",
			side:            types.SideTypeBuy,
			currentPrice:    Number(100.0),
			stopLossPrice:   Number(90.0),
			takeProfitPrice: Number(110.0),
			wantOK:          true,
		},
		{
			name:            "long invalid stop loss",
			side:            types.SideTypeBuy,
			currentPrice:    Number(100),
			stopLossPrice:   Number(105),
			takeProfitPrice: Number(110),
			wantOK:          false,
		},
		{
			name:            "long invalid take profit",
			side:            types.SideTypeBuy,
			currentPrice:    Number(100),
			stopLossPrice:   Number(90),
			takeProfitPrice: Number(95),
			wantOK:          false,
		},
		{
			name:            "short valid",
			side:            types.SideTypeSell,
			currentPrice:    Number(100),
			stopLossPrice:   Number(110),
			takeProfitPrice: Number(90),
			wantOK:          true,
		},
		{
			name:            "short invalid stop loss",
			side:            types.SideTypeSell,
			currentPrice:    Number(100),
			stopLossPrice:   Number(95),
			takeProfitPrice: Number(90),
			wantOK:          false,
		},
		{
			name:            "short invalid take profit",
			side:            types.SideTypeSell,
			currentPrice:    Number(100),
			stopLossPrice:   Number(110),
			takeProfitPrice: Number(105),
			wantOK:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := validateStopLossTakeProfit(tt.side, tt.currentPrice, tt.stopLossPrice, tt.takeProfitPrice)
			if ok != tt.wantOK {
				t.Errorf("validateStopLossTakeProfit() = %v, want %v, err: %v", ok, tt.wantOK, err)
			}
		})
	}
}
