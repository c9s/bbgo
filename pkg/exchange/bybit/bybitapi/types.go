package bybitapi

type Category string

const (
	CategorySpot Category = "spot"
)

type Status string

const (
	// StatusTrading is only include the "Trading" status for `spot` category.
	StatusTrading Status = "Trading"
)
