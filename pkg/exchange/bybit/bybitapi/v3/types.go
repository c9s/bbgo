package v3

type Side string

const (
	SideBuy  Side = "0"
	SideSell Side = "1"
)

type OrderType string

const (
	OrderTypeMaker OrderType = "0"
	OrderTypeTaker OrderType = "1"
)
