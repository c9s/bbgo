package style

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var LossEmoji = "🔥"
var ProfitEmoji = "💰"
var DefaultPnLLevelResolution = fixedpoint.NewFromFloat(0.001)

func PnLColor(pnl fixedpoint.Value) string {
	if pnl.Sign() > 0 {
		return GreenColor
	}
	return RedColor
}

func PnLSignString(pnl fixedpoint.Value) string {
	if pnl.Sign() > 0 {
		return "+" + pnl.String()
	}
	return pnl.String()
}

func PnLEmojiSimple(pnl fixedpoint.Value) string {
	if pnl.Sign() < 0 {
		return LossEmoji
	}

	if pnl.IsZero() {
		return ""
	}

	return ProfitEmoji
}

func PnLEmojiMargin(pnl, margin, resolution fixedpoint.Value) (out string) {
	if margin.IsZero() {
		return PnLEmojiSimple(pnl)
	}

	if pnl.Sign() < 0 {
		out = LossEmoji
		level := (margin.Neg()).Div(resolution).Int()
		for i := 1; i < level; i++ {
			out += LossEmoji
		}
		return out
	}

	if pnl.IsZero() {
		return out
	}

	out = ProfitEmoji
	level := margin.Div(resolution).Int()
	for i := 1; i < level; i++ {
		out += ProfitEmoji
	}
	return out
}
