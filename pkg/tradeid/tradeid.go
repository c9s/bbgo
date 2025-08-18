package tradeid

import (
	"sync/atomic"
	"time"
)

type Generator interface {
	Generate() uint64
}

type productionGenerator struct{}

var counter uint64

func (g *productionGenerator) Generate() uint64 {
	c := atomic.AddUint64(&counter, 1)
	ts := uint64(time.Now().UnixMilli())
	return ts*1000 + c
}

type deterministicGenerator struct {
	counter uint64
}

func (g *deterministicGenerator) Generate() uint64 {
	g.counter++
	return g.counter
}

var GlobalGenerator Generator = &productionGenerator{}

func NewDeterministicGenerator() Generator {
	return &deterministicGenerator{}
}
