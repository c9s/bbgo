package bbgo

import (
	"github.com/robfig/cron/v3"

	"github.com/c9s/bbgo/pkg/accounting/pnl"
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
