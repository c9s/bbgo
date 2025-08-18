package style

import (
	"strings"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var LossEmoji = "🔥"
var ProfitEmoji = "💰"

// 0.1% = 10 bps
var DefaultPnLLevelResolution = fixedpoint.NewFromFloat(0.001)

const MaxEmojiRepeat = 6

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
	if pnl.IsZero() {
		return ""
	}

	if pnl.Sign() < 0 {
		return LossEmoji
	} else {
		return ProfitEmoji
	}
}

// PnLEmojiMargin returns the emoji representation of the PnL with margin and resolution
func PnLEmojiMargin(pnl, margin, resolution fixedpoint.Value) string {
	if margin.IsZero() {
		return PnLEmojiSimple(pnl)
	}

	if pnl.IsZero() {
		return ""
	}

	if pnl.Sign() < 0 {
		level := min(margin.Abs().Div(resolution).Int(), MaxEmojiRepeat)
		return strings.Repeat(LossEmoji, level)
	}

	level := min(margin.Div(resolution).Int(), MaxEmojiRepeat)
	return strings.Repeat(ProfitEmoji, level)
}
