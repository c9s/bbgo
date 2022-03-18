package types

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var debugBalance = false

func init() {
	debugBalance = viper.GetBool("debug-balance")
}

type Balance struct {
	Currency  string           `json:"currency"`
	Available fixedpoint.Value `json:"available"`
	Locked    fixedpoint.Value `json:"locked,omitempty"`
}

func (b Balance) Total() fixedpoint.Value {
	return b.Available.Add(b.Locked)
}

func (b Balance) String() string {
	if b.Locked.Sign() > 0 {
		return fmt.Sprintf("%s: %v (locked %v)", b.Currency, b.Available, b.Locked)
	}

	return fmt.Sprintf("%s: %s", b.Currency, b.Available.String())
}

type Asset struct {
	Currency  string           `json:"currency" db:"currency"`
	Total     fixedpoint.Value `json:"total" db:"total"`
	InUSD     fixedpoint.Value `json:"inUSD" db:"inUSD"`
	InBTC     fixedpoint.Value `json:"inBTC" db:"inBTC"`
	Time      time.Time        `json:"time" db:"time"`
	Locked    fixedpoint.Value `json:"lock" db:"lock" `
	Available fixedpoint.Value `json:"available"  db:"available"`
}

type AssetMap map[string]Asset

func (m AssetMap) PlainText() (o string) {
	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].InUSD.Compare(assets[j].InUSD) > 0
	})

	sumUsd := fixedpoint.Zero
	sumBTC := fixedpoint.Zero
	for _, a := range assets {
		usd := a.InUSD
		btc := a.InBTC
		if !a.InUSD.IsZero() {
			o += fmt.Sprintf("  %s: %s (≈ %s) (≈ %s)",
				a.Currency,
				a.Total.String(),
				USD.FormatMoney(usd),
				BTC.FormatMoney(btc),
			) + "\n"
			sumUsd = sumUsd.Add(usd)
			sumBTC = sumBTC.Add(btc)
		} else {
			o += fmt.Sprintf("  %s: %s",
				a.Currency,
				a.Total.String(),
			) + "\n"
		}
	}
	o += fmt.Sprintf(" Summary: (≈ %s) (≈ %s)",
		USD.FormatMoney(sumUsd),
		BTC.FormatMoney(sumBTC),
	) + "\n"
	return o
}

func (m AssetMap) Slice() (assets []Asset) {
	for _, a := range m {
		assets = append(assets, a)
	}
	return assets
}

func (m AssetMap) SlackAttachment() slack.Attachment {
	var fields []slack.AttachmentField
	var totalBTC, totalUSD fixedpoint.Value

	var assets = m.Slice()

	// sort assets
	sort.Slice(assets, func(i, j int) bool {
		return assets[i].InUSD.Compare(assets[j].InUSD) > 0
	})

	for _, a := range assets {
		totalUSD = totalUSD.Add(a.InUSD)
		totalBTC = totalBTC.Add(a.InBTC)
	}

	for _, a := range assets {
		if !a.InUSD.IsZero() {
			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: fmt.Sprintf("%s (≈ %s) (≈ %s) (%s)",
					a.Total.String(),
					USD.FormatMoney(a.InUSD),
					BTC.FormatMoney(a.InBTC),
					a.InUSD.Div(totalUSD).FormatPercentage(2),
				),
				Short: false,
			})
		} else {
			fields = append(fields, slack.AttachmentField{
				Title: a.Currency,
				Value: fmt.Sprintf("%s", a.Total.String()),
				Short: false,
			})
		}
	}

	return slack.Attachment{
		Title: fmt.Sprintf("Net Asset Value %s (≈ %s)",
			USD.FormatMoney(totalUSD),
			BTC.FormatMoney(totalBTC),
		),
		Fields: fields,
	}
}

type BalanceMap map[string]Balance
type PositionMap map[string]Position
type IsolatedMarginAssetMap map[string]IsolatedMarginAsset
type MarginAssetMap map[string]MarginUserAsset
type FuturesAssetMap map[string]FuturesUserAsset
type FuturesPositionMap map[string]FuturesPosition

func (m BalanceMap) String() string {
	var ss []string
	for _, b := range m {
		ss = append(ss, b.String())
	}

	return "BalanceMap[" + strings.Join(ss, ", ") + "]"
}

func (m BalanceMap) Copy() (d BalanceMap) {
	d = make(BalanceMap)
	for c, b := range m {
		d[c] = b
	}
	return d
}

func (m BalanceMap) Assets(prices map[string]fixedpoint.Value) AssetMap {
	assets := make(AssetMap)

	now := time.Now()
	for currency, b := range m {
		if b.Locked.IsZero() && b.Available.IsZero() {
			continue
		}

		asset := Asset{
			Currency:  currency,
			Total:     b.Available.Add(b.Locked),
			Time:      now,
			Locked:    b.Locked,
			Available: b.Available,
		}

		btcusdt, hasBtcPrice := prices["BTCUSDT"]

		usdMarkets := []string{currency + "USDT", currency + "USDC", currency + "USD", "USDT" + currency}

		for _, market := range usdMarkets {
			if val, ok := prices[market]; ok {

				if strings.HasPrefix(market, "USD") {
					asset.InUSD = asset.Total.Div(val)
				} else {
					asset.InUSD = asset.Total.Mul(val)
				}

				if hasBtcPrice {
					asset.InBTC = asset.InUSD.Div(btcusdt)
				}
			}
		}

		assets[currency] = asset
	}

	return assets
}

func (m BalanceMap) Print() {
	for _, balance := range m {
		if balance.Available.IsZero() && balance.Locked.IsZero() {
			continue
		}

		if balance.Locked.Sign() > 0 {
			logrus.Infof(" %s: %v (locked %v)", balance.Currency, balance.Available, balance.Locked)
		} else {
			logrus.Infof(" %s: %v", balance.Currency, balance.Available)
		}
	}
}

type AccountType string

const (
	AccountTypeFutures = AccountType("futures")
	AccountTypeMargin  = AccountType("margin")
	AccountTypeSpot    = AccountType("spot")
)

type Account struct {
	sync.Mutex `json:"-"`

	AccountType        AccountType `json:"accountType,omitempty"`
	FuturesInfo        *FuturesAccountInfo
	MarginInfo         *MarginAccountInfo
	IsolatedMarginInfo *IsolatedMarginAccountInfo

	MakerFeeRate fixedpoint.Value `json:"makerFeeRate,omitempty"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate,omitempty"`

	TotalAccountValue fixedpoint.Value `json:"totalAccountValue,omitempty"`

	CanDeposit  bool `json:"canDeposit"`
	CanTrade    bool `json:"canTrade"`
	CanWithdraw bool `json:"canWithdraw"`

	balances BalanceMap
}

type FuturesAccountInfo struct {
	// Futures fields
	Assets                      FuturesAssetMap    `json:"assets"`
	Positions                   FuturesPositionMap `json:"positions"`
	TotalInitialMargin          fixedpoint.Value   `json:"totalInitialMargin"`
	TotalMaintMargin            fixedpoint.Value   `json:"totalMaintMargin"`
	TotalMarginBalance          fixedpoint.Value   `json:"totalMarginBalance"`
	TotalOpenOrderInitialMargin fixedpoint.Value   `json:"totalOpenOrderInitialMargin"`
	TotalPositionInitialMargin  fixedpoint.Value   `json:"totalPositionInitialMargin"`
	TotalUnrealizedProfit       fixedpoint.Value   `json:"totalUnrealizedProfit"`
	TotalWalletBalance          fixedpoint.Value   `json:"totalWalletBalance"`
	UpdateTime                  int64              `json:"updateTime"`
}

type MarginAccountInfo struct {
	// Margin fields
	BorrowEnabled       bool             `json:"borrowEnabled"`
	MarginLevel         fixedpoint.Value `json:"marginLevel"`
	TotalAssetOfBTC     fixedpoint.Value `json:"totalAssetOfBtc"`
	TotalLiabilityOfBTC fixedpoint.Value `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBTC  fixedpoint.Value `json:"totalNetAssetOfBtc"`
	TradeEnabled        bool             `json:"tradeEnabled"`
	TransferEnabled     bool             `json:"transferEnabled"`
	Assets              MarginAssetMap   `json:"userAssets"`
}

type IsolatedMarginAccountInfo struct {
	TotalAssetOfBTC     fixedpoint.Value       `json:"totalAssetOfBtc"`
	TotalLiabilityOfBTC fixedpoint.Value       `json:"totalLiabilityOfBtc"`
	TotalNetAssetOfBTC  fixedpoint.Value       `json:"totalNetAssetOfBtc"`
	Assets              IsolatedMarginAssetMap `json:"userAssets"`
}

func NewAccount() *Account {
	return &Account{
		balances: make(BalanceMap),
	}
}

// Balances lock the balances and returned the copied balances
func (a *Account) Balances() (d BalanceMap) {
	a.Lock()
	d = a.balances.Copy()
	a.Unlock()
	return d
}

func (a *Account) Balance(currency string) (balance Balance, ok bool) {
	a.Lock()
	balance, ok = a.balances[currency]
	a.Unlock()
	return balance, ok
}

func (a *Account) AddBalance(currency string, fund fixedpoint.Value) {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok {
		balance.Available = balance.Available.Add(fund)
		a.balances[currency] = balance
		return
	}

	a.balances[currency] = Balance{
		Currency:  currency,
		Available: fund,
		Locked:    fixedpoint.Zero,
	}
}

func (a *Account) UseLockedBalance(currency string, fund fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok && balance.Locked.Compare(fund) >= 0 {
		balance.Locked = balance.Locked.Sub(fund)
		a.balances[currency] = balance
		return nil
	}

	return fmt.Errorf("trying to use more than locked: locked %v < want to use %v", balance.Locked, fund)
}

var QuantityDelta = fixedpoint.MustNewFromString("0.00000000001")

func (a *Account) UnlockBalance(currency string, unlocked fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if !ok {
		return fmt.Errorf("trying to unlocked inexisted balance: %s", currency)
	}

	// Instead of showing error in UnlockBalance,
	// since this function is only called when cancel orders,
	// there might be inequivalence in the last order quantity
	if unlocked.Compare(balance.Locked) > 0 {
		// check if diff is within delta
		if unlocked.Sub(balance.Locked).Compare(QuantityDelta) <= 0 {
			balance.Available = balance.Available.Add(balance.Locked)
			balance.Locked = fixedpoint.Zero
			a.balances[currency] = balance
			return nil
		}
		return fmt.Errorf("trying to unlocked more than locked %s: locked %v < want to unlock %v", currency, balance.Locked, unlocked)
	}

	balance.Locked = balance.Locked.Sub(unlocked)
	balance.Available = balance.Available.Add(unlocked)
	a.balances[currency] = balance
	return nil
}

func (a *Account) LockBalance(currency string, locked fixedpoint.Value) error {
	a.Lock()
	defer a.Unlock()

	balance, ok := a.balances[currency]
	if ok && balance.Available.Compare(locked) >= 0 {
		balance.Locked = balance.Locked.Add(locked)
		balance.Available = balance.Available.Sub(locked)
		a.balances[currency] = balance
		return nil
	}

	return fmt.Errorf("insufficient available balance %s for lock: want to lock %v, available %v", currency, locked, balance.Available)
}

func (a *Account) UpdateBalances(balances BalanceMap) {
	a.Lock()
	defer a.Unlock()

	if a.balances == nil {
		a.balances = make(BalanceMap)
	}

	for _, balance := range balances {
		a.balances[balance.Currency] = balance
	}
}

func printBalanceUpdate(balances BalanceMap) {
	logrus.Infof("balance update: %+v", balances)
}

func (a *Account) BindStream(stream Stream) {
	stream.OnBalanceUpdate(a.UpdateBalances)
	stream.OnBalanceSnapshot(a.UpdateBalances)
	if debugBalance {
		stream.OnBalanceUpdate(printBalanceUpdate)
	}
}

func (a *Account) Print() {
	a.Lock()
	defer a.Unlock()

	if a.AccountType != "" {
		logrus.Infof("account type: %s", a.AccountType)
	}

	if a.MakerFeeRate.Sign() > 0 {
		logrus.Infof("maker fee rate: %v", a.MakerFeeRate)
	}
	if a.TakerFeeRate.Sign() > 0 {
		logrus.Infof("taker fee rate: %v", a.TakerFeeRate)
	}

	a.balances.Print()
}
