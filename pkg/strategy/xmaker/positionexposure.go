package xmaker

import "github.com/c9s/bbgo/pkg/fixedpoint"

//go:generate callbackgen -type PositionExposure
type PositionExposure struct {
	symbol string

	// net = net position
	// pending = covered position
	net, pending fixedpoint.MutexValue

	coverCallbacks []func(d fixedpoint.Value)
	closeCallbacks []func(d fixedpoint.Value)
}

func newPositionExposure(symbol string) *PositionExposure {
	return &PositionExposure{
		symbol: symbol,
	}
}

func (m *PositionExposure) Open(delta fixedpoint.Value) {
	m.net.Add(delta)

	log.Infof(
		"%s opened:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)
}

func (m *PositionExposure) Cover(delta fixedpoint.Value) {
	m.pending.Add(delta)

	log.Infof(
		"%s covered:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)

	m.EmitCover(delta)
}

func (m *PositionExposure) Close(delta fixedpoint.Value) {
	m.pending.Add(delta)
	m.net.Add(delta)

	log.Infof(
		"%s closed:%f netPosition:%f coveredPosition: %f",
		m.symbol,
		delta.Float64(),
		m.net.Get().Float64(),
		m.pending.Get().Float64(),
	)

	m.EmitClose(delta)
}

func (m *PositionExposure) IsClosed() bool {
	return m.net.Get().IsZero() && m.pending.Get().IsZero()
}

func (m *PositionExposure) GetUncovered() fixedpoint.Value {
	netPosition := m.net.Get()
	coveredPosition := m.pending.Get()
	uncoverPosition := netPosition.Sub(coveredPosition)
	return uncoverPosition
}
