package csvsource

import (
	"time"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type CsvTick struct {
	Timestamp       int64            `json:"timestamp"`
	Symbol          string           `json:"symbol"`
	Side            string           `json:"side"`
	TickDirection   string           `json:"tickDirection"`
	Size            fixedpoint.Value `json:"size"`
	Price           fixedpoint.Value `json:"price"`
	HomeNotional    fixedpoint.Value `json:"homeNotional"`
	ForeignNotional fixedpoint.Value `json:"foreignNotional"`
}

type SupportedExchange int

const (
	Bybit   SupportedExchange = 0
	Binance SupportedExchange = 1
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

func convertInterval(kInterval KLineInterval) time.Duration {
	var interval = time.Minute
	switch kInterval {
	case M1:
	case M5:
		return time.Minute * 5
	case M15:
		return time.Minute * 15
	case M30:
		return time.Minute * 30
	case H1:
		return time.Hour
	case H2:
		return time.Hour * 2
	case H4:
		return time.Hour * 4
	case D1:
		return time.Hour * 24
	}
	return interval
}

func convertTimestamp(ts time.Time, kInterval KLineInterval) time.Time {
	var start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), ts.Minute(), 0, 0, ts.Location())
	switch kInterval {
	case M1: // default start as defined above
	case M5:
		minute := ts.Minute() - (ts.Minute() % 5)
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), minute, 0, 0, ts.Location())
	case M15:
		minute := ts.Minute() - (ts.Minute() % 15)
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), minute, 0, 0, ts.Location())
	case M30:
		minute := ts.Minute() - (ts.Minute() % 30)
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), minute, 0, 0, ts.Location())
	case H1:
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
	case H2:
		hour := ts.Hour() - (ts.Hour() % 2)
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), hour, 0, 0, 0, ts.Location())
	case H4:
		hour := ts.Hour() - (ts.Hour() % 4)
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), hour, 0, 0, 0, ts.Location())
	case D1:
		start = time.Date(ts.Year(), ts.Month(), ts.Day(), 0, 0, 0, 0, ts.Location())
	}
	return start
}
