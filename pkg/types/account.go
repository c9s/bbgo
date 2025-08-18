package types

import (
	"fmt"
	"sync"

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
	AccountTypeCredit         = AccountType("credit")
)

type Account struct {
	sync.Mutex `json:"-"`

	AccountType        AccountType `json:"accountType,omitempty"`
	FuturesInfo        *FuturesAccount
	MarginInfo         *MarginAccountInfo
	IsolatedMarginInfo *IsolatedMarginAccountInfo

	// Margin related common field
	// From binance:
	// Margin Level = Total Asset Value / (Total Borrowed + Total Accrued Interest)
	//
	// If your margin level drops to 1.3, you will receive a Margin Call, which is a reminder that you should either increase your collateral (by depositing more funds) or reduce your loan (by repaying what you’ve borrowed).
	// If your margin level drops to 1.1, your assets will be automatically liquidated, meaning that Binance will sell your funds at market price to repay the loan.
	MarginLevel fixedpoint.Value `json:"marginLevel,omitempty"`

	MarginTolerance fixedpoint.Value `json:"marginTolerance,omitempty"`

	// MarginRatio = Adjusted equity / (Maintenance margin + Liquidation fees)
	MarginRatio fixedpoint.Value `json:"marginRatio,omitempty"`

	BorrowEnabled   *bool `json:"borrowEnabled,omitempty"`
	TransferEnabled *bool `json:"transferEnabled,omitempty"`

	// Isolated margin related fields
	// ------------------------------
	// LiquidationPrice is only used when account is in the isolated margin mode
	LiquidationPrice fixedpoint.Value `json:"liquidationPrice,omitempty"`
	LiquidationRate  fixedpoint.Value `json:"liquidationRate,omitempty"`

	MakerFeeRate fixedpoint.Value `json:"makerFeeRate,omitempty"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate,omitempty"`

	// TotalAccountValue is the total value of the account in USD or USDT
	TotalAccountValue fixedpoint.Value `json:"totalAccountValue,omitempty"`

	CanDeposit  bool `json:"canDeposit"`
	CanTrade    bool `json:"canTrade"`
	CanWithdraw bool `json:"canWithdraw"`

	balances BalanceMap
}

// FuturesAccount defines the account information for futures trading
// Mostly are derived from the Binance API response
//
// @see https://www.binance.com/en/support/faq/detail/b3c689c1f50a44cabb3a84e663b81d93?hl=en
type FuturesAccount struct {
	// Futures fields
	Assets    FuturesAssetMap    `json:"assets"`
	Positions FuturesPositionMap `json:"positions"`

	// Total initial margin required with current mark price (useless with isolated positions), only for USDT asset
	//
	// Initial Margin = Quantity * Entry Price * IMR
	// IMR = 1 / leverage
	TotalInitialMargin fixedpoint.Value `json:"totalInitialMargin"`

	// Total maintenance margin required, only for USDT asset
	//
	// Maintenance Margin = Position Notional * Maintenance Margin Rate on the level of position notional -  Maintenance Amount on the level of position notional
	TotalMaintMargin fixedpoint.Value `json:"totalMaintMargin"`

	// Total wallet balance, only for USDT asset
	//
	// Wallet Balance = Total Net Transfer + Total Realized Profit + Total Net Funding Fee - Total Commission
	TotalWalletBalance fixedpoint.Value `json:"totalWalletBalance"`

	// Total margin balance, only for USDT asset
	//
	// Margin Balance = Wallet Balance + Unrealized PNL.
	// Your positions will be liquidated once the Margin Balance ≤ the Maintenance Margin.
	TotalMarginBalance fixedpoint.Value `json:"totalMarginBalance"`

	// TotalOpenOrderInitialMargin
	//
	// For USDⓈ-M Futures:
	// Available for Order =
	//		max(0,
	//			crossWalletBalance + ∑cross Unrealized PNL - (∑Cross Initial Margin + ∑Isolated Open Order Initial Margin)
	//		)
	// Isolated Open Order Initial Margin = abs(Isolated Present Notional) * IMR - abs(Size) * IMR * Mark Price
	TotalOpenOrderInitialMargin fixedpoint.Value `json:"totalOpenOrderInitialMargin"`

	TotalPositionInitialMargin fixedpoint.Value `json:"totalPositionInitialMargin"`
	TotalUnrealizedProfit      fixedpoint.Value `json:"totalUnrealizedProfit"`

	// AvailableBalance
	AvailableBalance fixedpoint.Value `json:"availableBalance"`
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
		BorrowEnabled:      BoolPtr(false),
		TransferEnabled:    BoolPtr(false),
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
	if !ok {
		balance = NewZeroBalance(currency)
	}

	return balance, ok
}

func (a *Account) SetBalance(currency string, bal Balance) {
	a.Lock()
	defer a.Unlock()

	bal.Currency = currency
	a.balances[currency] = bal
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

func (a *Account) Print(log LogFunc) {
	a.Lock()
	defer a.Unlock()

	if a.AccountType != "" {
		log("AccountType: %s", a.AccountType)
	}

	switch a.AccountType {
	case AccountTypeMargin, AccountTypeFutures:
		if a.MarginLevel.Sign() != 0 {
			log("MarginLevel: %f", a.MarginLevel.Float64())
		}

		if a.MarginTolerance.Sign() != 0 {
			log("MarginTolerance: %f", a.MarginTolerance.Float64())
		}

		if a.BorrowEnabled != nil {
			log("BorrowEnabled: %v", *a.BorrowEnabled)
		}

		if a.MarginInfo != nil {
			log("MarginInfo: %#v", a.MarginInfo)
		}
	}

	if a.TransferEnabled != nil {
		log("TransferEnabled: %v", *a.TransferEnabled)
	}

	if a.MakerFeeRate.Sign() != 0 {
		log("MakerFeeRate: %v", a.MakerFeeRate)
	}

	if a.TakerFeeRate.Sign() != 0 {
		log("TakerFeeRate: %v", a.TakerFeeRate)
	}

	a.balances.Print(log)
}
