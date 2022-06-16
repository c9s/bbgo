package statistics

import (
	"github.com/c9s/bbgo/pkg/types"
)

// Sharpe: Calcluates the sharpe ratio of access returns
//
// @param rf (float): Risk-free rate expressed as a yearly (annualized) return
// @param periods (int): Freq. of returns (252/365 for daily, 12 for monthy)
// @param annualize (bool): return annualize sharpe?
// @param smart (bool): return smart sharpe ratio
func Sharpe(returns types.Series, rf float64, periods int, annualize bool, smart bool) {
}
