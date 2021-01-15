package binance

type MarginSettings struct {
	useMargin               bool
	useMarginIsolated       bool
	useMarginIsolatedSymbol string
}

func (e *MarginSettings) UseMargin() {
	e.useMargin = true
}

func (e *MarginSettings) UseIsolatedMargin(symbol string) {
	e.useMargin = true
	e.useMarginIsolated = true
	e.useMarginIsolatedSymbol = symbol
}
