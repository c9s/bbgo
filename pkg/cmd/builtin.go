package cmd

// import built-in strategies
import (
	_ "github.com/c9s/bbgo/pkg/strategy/bollgrid"
	_ "github.com/c9s/bbgo/pkg/strategy/buyandhold"
	_ "github.com/c9s/bbgo/pkg/strategy/flashcrash"
	_ "github.com/c9s/bbgo/pkg/strategy/gap"
	_ "github.com/c9s/bbgo/pkg/strategy/grid"
	_ "github.com/c9s/bbgo/pkg/strategy/pricealert"
	_ "github.com/c9s/bbgo/pkg/strategy/schedule"
	_ "github.com/c9s/bbgo/pkg/strategy/support"
	_ "github.com/c9s/bbgo/pkg/strategy/swing"
	_ "github.com/c9s/bbgo/pkg/strategy/trailingstop"
	
	_ "github.com/c9s/bbgo/pkg/strategy/xbalance"
	_ "github.com/c9s/bbgo/pkg/strategy/xmaker"
	_ "github.com/c9s/bbgo/pkg/strategy/xpuremaker"
)
