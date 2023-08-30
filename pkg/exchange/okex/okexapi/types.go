package okexapi

type Category string

const (
	CategoryTwap               Category = "twap"
	CategoryAdl                Category = "adl"
	CategoryFullLiquidation    Category = "full_liquidation"
	CategoryPartialLiquidation Category = "partial_liquidation"
	CategoryDelivery           Category = "delivery"
	CategoryDdh                Category = "ddh"
)
