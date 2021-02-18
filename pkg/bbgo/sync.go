package bbgo

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
)

func stringSliceContains(slice []string, needle string) bool {
	for _, s := range slice {
		if s == needle {
			return true
		}
	}

	return false
}

func FindPossibleSymbols(session *ExchangeSession) (symbols []string, err error) {
	var balances = session.Account.Balances()
	var fiatCurrencies = []string{"USDC", "USDT", "USD", "TWD", "EUR", "GBP"}
	var fiatAssets []string

	for _, currency := range fiatCurrencies {
		if balance, ok := balances[currency]; ok && balance.Total() > 0 {
			fiatAssets = append(fiatAssets, currency)
		}
	}

	var symbolMap = map[string]struct{}{}

	for _, market := range session.Markets() {
		// ignore the markets that are not fiat currency markets
		if !stringSliceContains(fiatAssets, market.QuoteCurrency) {
			continue
		}

		// ignore the asset that we don't have in the balance sheet
		balance, hasAsset := balances[market.BaseCurrency]
		if !hasAsset || balance.Total() == 0 {
			continue
		}

		symbolMap[market.Symbol] = struct{}{}
	}

	for s := range symbolMap {
		symbols = append(symbols, s)
	}

	return symbols, nil
}

// SyncSessionSymbols syncs the trades from the given exchange session
func SyncSessionSymbols(ctx context.Context, environ *Environment, session *ExchangeSession, startTime time.Time, symbols ...string) error {
	for _, symbol := range symbols {
		logrus.Debugf("syncing trades from exchange session %s...", session.Name)
		if err := environ.TradeSync.SyncTrades(ctx, session.Exchange, symbol, startTime); err != nil {
			return err
		}

		logrus.Debugf("syncing orders from exchange session %s...", session.Name)
		if err := environ.TradeSync.SyncOrders(ctx, session.Exchange, symbol, startTime); err != nil {
			return err
		}
	}

	return nil
}

func getSessionSymbols(session *ExchangeSession, defaultSymbols ...string) ([]string, error) {
	if session.IsolatedMargin {
		return []string{session.IsolatedMarginSymbol}, nil
	}

	if len(defaultSymbols) > 0 {
		return defaultSymbols, nil
	}

	return FindPossibleSymbols(session)
}

func SyncSession(ctx context.Context, environ *Environment, session *ExchangeSession, startTime time.Time, defaultSymbols ...string) error {
	symbols, err := getSessionSymbols(session, defaultSymbols...)
	if err != nil {
		return err
	}

	return SyncSessionSymbols(ctx, environ, session, startTime, symbols...)
}
