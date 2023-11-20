package types

import (
	"fmt"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

var debugBalance = false

func init() {
	debugBalance = viper.GetBool("debug-balance")
}

type PositionMap map[string]Position
type IsolatedMarginAssetMap map[string]IsolatedMarginAsset
type MarginAssetMap map[string]MarginUserAsset
type FuturesAssetMap map[string]FuturesUserAsset
type FuturesPositionMap map[string]FuturesPosition

type AccountType string

const (
	AccountTypeFutures        = AccountType("futures")
	AccountTypeMargin         = AccountType("margin")
	AccountTypeIsolatedMargin = AccountType("isolated_margin")
	AccountTypeSpot           = AccountType("spot")
)

type Account struct {
	sync.Mutex `json:"-"`

	AccountType        AccountType `json:"accountType,omitempty"`
	FuturesInfo        *FuturesAccountInfo
	MarginInfo         *MarginAccountInfo
	IsolatedMarginInfo *IsolatedMarginAccountInfo

	// Margin related common field
	// From binance:
	// Margin Level = Total Asset Value / (Total Borrowed + Total Accrued Interest)
	// If your margin level drops to 1.3, you will receive a Margin Call, which is a reminder that you should either increase your collateral (by depositing more funds) or reduce your loan (by repaying what you’ve borrowed).
	// If your margin level drops to 1.1, your assets will be automatically liquidated, meaning that Binance will sell your funds at market price to repay the loan.
	MarginLevel     fixedpoint.Value `json:"marginLevel,omitempty"`
	MarginTolerance fixedpoint.Value `json:"marginTolerance,omitempty"`

	BorrowEnabled   bool `json:"borrowEnabled,omitempty"`
	TransferEnabled bool `json:"transferEnabled,omitempty"`

	// isolated margin related fields
	// LiquidationPrice is only used when account is in the isolated margin mode
	MarginRatio      fixedpoint.Value `json:"marginRatio,omitempty"`
	LiquidationPrice fixedpoint.Value `json:"liquidationPrice,omitempty"`
	LiquidationRate  fixedpoint.Value `json:"liquidationRate,omitempty"`

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
		AccountType:        "spot",
		FuturesInfo:        nil,
		MarginInfo:         nil,
		IsolatedMarginInfo: nil,
		MarginLevel:        fixedpoint.Zero,
		MarginTolerance:    fixedpoint.Zero,
		BorrowEnabled:      false,
		TransferEnabled:    false,
		MarginRatio:        fixedpoint.Zero,
		LiquidationPrice:   fixedpoint.Zero,
		LiquidationRate:    fixedpoint.Zero,
		MakerFeeRate:       fixedpoint.Zero,
		TakerFeeRate:       fixedpoint.Zero,
		TotalAccountValue:  fixedpoint.Zero,
		CanDeposit:         false,
		CanTrade:           false,
		CanWithdraw:        false,
		balances:           make(BalanceMap),
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
	if !ok {
		return fmt.Errorf("account balance %s does not exist", currency)
	}

	// simple case, using fund less than locked
	if balance.Locked.Compare(fund) >= 0 {
		balance.Locked = balance.Locked.Sub(fund)
		a.balances[currency] = balance
		return nil
	}

	return fmt.Errorf("trying to use more than locked: locked %v < want to use %v diff %v", balance.Locked, fund, balance.Locked.Sub(fund))
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
