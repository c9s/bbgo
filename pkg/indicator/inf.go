package indicator

import "github.com/c9s/bbgo/pkg/types"

type KLineWindowUpdater interface {
	OnKLineWindowUpdate(func(interval types.Interval, window types.KLineWindow))
}

type KLineCloseHandler interface {
	OnKLineClosed(func(k types.KLine))
}
