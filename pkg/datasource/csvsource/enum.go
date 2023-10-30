package csvsource

type SupportedExchange int

const (
	Bybit SupportedExchange = 0
)

type KLineInterval string

const (
	M1  KLineInterval = "1m"
	M5  KLineInterval = "5m"
	M15 KLineInterval = "15m"
	M30 KLineInterval = "30m"
	H1  KLineInterval = "1h"
	H2  KLineInterval = "2h"
	H4  KLineInterval = "4h"
	D1  KLineInterval = "1d"
)
