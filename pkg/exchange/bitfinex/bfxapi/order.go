package bfxapi

type OrderFlag int

const (
	OrderFlagHidden   OrderFlag = 64
	OrderFlagClose    OrderFlag = 512
	OrderFlagPostOnly OrderFlag = 4096
	OrderFlagOCO      OrderFlag = 16384
)
