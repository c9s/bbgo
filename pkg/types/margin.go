package types

type MarginExchange interface {
	UseMargin()
	UseIsolatedMargin(symbol string)
	GetMarginSettings() MarginSettings
	// QueryMarginAccount(ctx context.Context) (*binance.MarginAccount, error)
}

type MarginSettings struct {
	IsMargin             bool
	IsIsolatedMargin     bool
	IsolatedMarginSymbol string
}

func (e MarginSettings) GetMarginSettings() MarginSettings {
	return e
}

func (e *MarginSettings) UseMargin() {
	e.IsMargin = true
}

func (e *MarginSettings) UseIsolatedMargin(symbol string) {
	e.IsMargin = true
	e.IsIsolatedMargin = true
	e.IsolatedMarginSymbol = symbol
}
