package bbgo

import (
	"regexp"

	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
	"github.com/c9s/bbgo/pkg/types"
	"github.com/c9s/bbgo/pkg/util"
)

type PnLReporter interface {
	Run()
}

type baseReporter struct {
	notifier    Notifier
	cron        *cron.Cron
	environment *Environment
}

type PnLReporterManager struct {
	baseReporter

	reporters []PnLReporter
}

func NewPnLReporter(notifier Notifier) *PnLReporterManager {
	return &PnLReporterManager{
		baseReporter: baseReporter{
			notifier: notifier,
			cron:     cron.New(),
		},
	}
}

func (manager *PnLReporterManager) AverageCostBySymbols(symbols ...string) *AverageCostPnLReporter {
	reporter := &AverageCostPnLReporter{
		baseReporter: manager.baseReporter,
		Symbols:      symbols,
	}

	manager.reporters = append(manager.reporters, reporter)
	return reporter
}

type AverageCostPnLReporter struct {
	baseReporter

	Sessions []string
	Symbols  []string
}

func (reporter *AverageCostPnLReporter) Of(sessions ...string) *AverageCostPnLReporter {
	reporter.Sessions = sessions
	return reporter
}

func (reporter *AverageCostPnLReporter) When(specs ...string) *AverageCostPnLReporter {
	for _, spec := range specs {
		_, err := reporter.cron.AddJob(spec, reporter)
		if err != nil {
			panic(err)
		}
	}

	return reporter
}

func (reporter *AverageCostPnLReporter) Run() {
	for _, sessionName := range reporter.Sessions {
		session := reporter.environment.sessions[sessionName]
		calculator := &pnl.AverageCostCalculator{
			TradingFeeCurrency: session.Exchange.PlatformFeeCurrency(),
		}

		for _, symbol := range reporter.Symbols {
			report := calculator.Calculate(symbol, session.Trades[symbol], session.lastPrices[symbol])
			report.Print()
		}
	}
}

type SymbolChannelRouter struct {
	routes map[*regexp.Regexp]string
}

func (router *SymbolChannelRouter) RouteSymbols(routes map[string]string) *SymbolChannelRouter {
	for pattern, channel := range routes {
		router.routes[regexp.MustCompile(pattern)] = channel
	}

	return router
}

func (router *SymbolChannelRouter) Dispatch(symbol string) (channel string, ok bool) {
	for pattern, channel := range router.routes {
		if pattern.MatchString(symbol) {
			ok = true
			return channel, ok
		}
	}

	return channel, ok
}

type ObjectChannelHandler func(obj interface{}) (channel string, ok bool)

type ObjectChannelRouter struct {
	routes []ObjectChannelHandler
}

func (router *ObjectChannelRouter) Route(f ObjectChannelHandler) *ObjectChannelRouter {
	router.routes = append(router.routes, f)
	return router
}

func (router *ObjectChannelRouter) Dispatch(obj interface{}) (channel string, ok bool) {
	for _, f := range router.routes {
		channel, ok = f(obj)
		if ok {
			return
		}
	}
	return
}

type TradeReporter struct {
	notifier Notifier

	channel       string
	channelRoutes map[*regexp.Regexp]string
}

func NewTradeReporter(notifier Notifier) *TradeReporter {
	return &TradeReporter{
		notifier:      notifier,
		channelRoutes: make(map[*regexp.Regexp]string),
	}
}

func (reporter *TradeReporter) Channel(channel string) *TradeReporter {
	reporter.channel = channel
	return reporter
}

func (reporter *TradeReporter) ChannelBySymbol(routes map[string]string) *TradeReporter {
	for pattern, channel := range routes {
		reporter.channelRoutes[regexp.MustCompile(pattern)] = channel
	}

	return reporter
}

func (reporter *TradeReporter) getChannel(symbol string) string {
	for pattern, channel := range reporter.channelRoutes {
		if pattern.MatchString(symbol) {
			return channel
		}
	}

	return reporter.channel
}

func (reporter *TradeReporter) Report(trade types.Trade) {
	var channel = reporter.getChannel(trade.Symbol)

	var text = util.Render(`:handshake: {{ .Symbol }} {{ .Side }} Trade Execution @ {{ .Price  }}`, trade)
	if err := reporter.notifier.NotifyTo(channel, text, trade); err != nil {
		logrus.WithError(err).Errorf("notifier error, channel=%s", channel)
	}
}
