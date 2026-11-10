package bbgo

import (
	"github.com/c9s/bbgo/pkg/rpcexport"
)

// CoreRPC is the RPC analog of CoreInteraction: constructed once per
// Trader (not per strategy), it registers framework-owned methods onto a
// shared rpcexport.Registry so they become remotely invocable over gRPC.
//
// ExportRPC currently registers only a demo method proving the mechanism
// end-to-end. Real framework-control methods (listing sessions, querying
// positions, etc.) are meant to be added here incrementally over time,
// the same way CoreInteraction's commands grew — that's deliberately not
// part of this change.
type CoreRPC struct {
	environment *Environment
	trader      *Trader
}

func NewCoreRPC(environment *Environment, trader *Trader) *CoreRPC {
	return &CoreRPC{
		environment: environment,
		trader:      trader,
	}
}

func (c *CoreRPC) ExportRPC(reg *rpcexport.Registry) {
	reg.Export("ping", "demo core RPC handler: returns \"pong\"", false, func() (string, error) {
		return "pong", nil
	})
}
