package autobuy

type PriceType string

const (
	PriceTypeLast = PriceType("last")
	PriceTypeBuy  = PriceType("buy")
	PriceTypeSell = PriceType("sell")
	PriceTypeMid  = PriceType("mid")
)
