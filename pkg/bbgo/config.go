package bbgo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/dynamic"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/service"
	"github.com/c9s/bbgo/pkg/types"
)

// DefaultFeeRate set the fee rate for most cases
// BINANCE uses 0.1% for both maker and taker
//
//	for BNB holders, it's 0.075% for both maker and taker
//
// MAX uses 0.050% for maker and 0.15% for taker
var DefaultFeeRate = fixedpoint.NewFromFloat(0.075 * 0.01)

type PnLReporterConfig struct {
	AverageCostBySymbols datatype.StringSlice `json:"averageCostBySymbols" yaml:"averageCostBySymbols"`
	Of                   datatype.StringSlice `json:"of" yaml:"of"`
	When                 datatype.StringSlice `json:"when" yaml:"when"`
}

// ExchangeStrategyMount wraps the SingleExchangeStrategy with the ExchangeSession name for mounting
type ExchangeStrategyMount struct {
	// Mounts contains the ExchangeSession name to mount
	Mounts []string `json:"mounts"`

	// Strategy is the strategy we loaded from config
	Strategy SingleExchangeStrategy `json:"strategy"`
}

func (m *ExchangeStrategyMount) Map() (map[string]interface{}, error) {
	strategyID := m.Strategy.ID()

	var params map[string]interface{}

	out, err := json.Marshal(m.Strategy)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(out, &params); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"on":       m.Mounts,
		strategyID: params,
	}, nil
}

type SlackNotification struct {
	DefaultChannel string `json:"defaultChannel,omitempty"  yaml:"defaultChannel,omitempty"`
	ErrorChannel   string `json:"errorChannel,omitempty"  yaml:"errorChannel,omitempty"`
}

type SlackNotificationRouting struct {
	Trade       string `json:"trade,omitempty" yaml:"trade,omitempty"`
	Order       string `json:"order,omitempty" yaml:"order,omitempty"`
	SubmitOrder string `json:"submitOrder,omitempty" yaml:"submitOrder,omitempty"`
	PnL         string `json:"pnL,omitempty" yaml:"pnL,omitempty"`
}

type TelegramNotification struct {
	Broadcast bool `json:"broadcast" yaml:"broadcast"`
}

type NotificationSwitches struct {
	Trade       bool `json:"trade" yaml:"trade"`
	Position    bool `json:"position" yaml:"position"`
	OrderUpdate bool `json:"orderUpdate" yaml:"orderUpdate"`
	SubmitOrder bool `json:"submitOrder" yaml:"submitOrder"`
}

type NotificationConfig struct {
	Slack    *SlackNotification    `json:"slack,omitempty" yaml:"slack,omitempty"`
	Telegram *TelegramNotification `json:"telegram,omitempty" yaml:"telegram,omitempty"`
	Switches *NotificationSwitches `json:"switches" yaml:"switches"`
}

type LoggingConfig struct {
	Trade           bool                   `json:"trade,omitempty"`
	Order           bool                   `json:"order,omitempty"`
	Balance         bool                   `json:"balance,omitempty"`
	FilledOrderOnly bool                   `json:"filledOrder,omitempty"`
	Fields          map[string]interface{} `json:"fields,omitempty"`
}

type Session struct {
	Name         string `json:"name,omitempty" yaml:"name,omitempty"`
	ExchangeName string `json:"exchange" yaml:"exchange"`
	EnvVarPrefix string `json:"envVarPrefix" yaml:"envVarPrefix"`

	Key    string `json:"key,omitempty" yaml:"key,omitempty"`
	Secret string `json:"secret,omitempty" yaml:"secret,omitempty"`

	PublicOnly           bool   `json:"publicOnly,omitempty" yaml:"publicOnly"`
	Margin               bool   `json:"margin,omitempty" yaml:"margin,omitempty"`
	IsolatedMargin       bool   `json:"isolatedMargin,omitempty" yaml:"isolatedMargin,omitempty"`
	IsolatedMarginSymbol string `json:"isolatedMarginSymbol,omitempty" yaml:"isolatedMarginSymbol,omitempty"`
}

//go:generate go run github.com/dmarkham/enumer -type=BacktestFeeMode -transform=snake -trimprefix BacktestFeeMode -yaml -json
type BacktestFeeMode int

const (
	// BackTestFeeModeQuoteFee is designed for clean position but which also counts the fee in the quote balance.
	// buy order = quote currency fee
	// sell order = quote currency fee
	BacktestFeeModeQuote BacktestFeeMode = iota // quote

	// BackTestFeeModeNativeFee is the default crypto exchange fee mode.
	// buy order = base currency fee
	// sell order = quote currency fee
	BacktestFeeModeNative // BackTestFeeMode = "native"

	// BackTestFeeModeFeeToken is the mode which calculates fee from the outside of the balances.
	// the fee will not be included in the balances nor the profit.
	BacktestFeeModeToken // BackTestFeeMode = "token"
)

type Backtest struct {
	StartTime types.LooseFormatTime  `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	EndTime   *types.LooseFormatTime `json:"endTime,omitempty" yaml:"endTime,omitempty"`

	// RecordTrades is an option, if set to true, back-testing should record the trades into database
	RecordTrades bool `json:"recordTrades,omitempty" yaml:"recordTrades,omitempty"`

	// Deprecated:
	// Account is deprecated, use Accounts instead
	Account map[string]BacktestAccount `json:"account" yaml:"account"`

	FeeMode BacktestFeeMode `json:"feeMode" yaml:"feeMode"`

	Accounts map[string]BacktestAccount `json:"accounts" yaml:"accounts"`
	Symbols  []string                   `json:"symbols" yaml:"symbols"`
	Sessions []string                   `json:"sessions" yaml:"sessions"`

	// sync 1 second interval KLines
	SyncSecKLines bool `json:"syncSecKLines,omitempty" yaml:"syncSecKLines,omitempty"`
}

func (b *Backtest) GetAccount(n string) BacktestAccount {
	accountConfig, ok := b.Accounts[n]
	if ok {
		return accountConfig
	}

	accountConfig, ok = b.Account[n]
	if ok {
		return accountConfig
	}

	return DefaultBacktestAccount
}

type BacktestAccount struct {
	MakerFeeRate fixedpoint.Value `json:"makerFeeRate,omitempty" yaml:"makerFeeRate,omitempty"`
	TakerFeeRate fixedpoint.Value `json:"takerFeeRate,omitempty" yaml:"takerFeeRate,omitempty"`

	Balances BacktestAccountBalanceMap `json:"balances" yaml:"balances"`
}

var DefaultBacktestAccount = BacktestAccount{
	MakerFeeRate: fixedpoint.MustNewFromString("0.050%"),
	TakerFeeRate: fixedpoint.MustNewFromString("0.075%"),
	Balances: BacktestAccountBalanceMap{
		"USDT": fixedpoint.NewFromFloat(10000),
	},
}

type BA BacktestAccount

func (b *BacktestAccount) UnmarshalYAML(value *yaml.Node) error {
	bb := &BA{MakerFeeRate: DefaultFeeRate, TakerFeeRate: DefaultFeeRate}
	if err := value.Decode(bb); err != nil {
		return err
	}
	*b = BacktestAccount(*bb)
	return nil
}

func (b *BacktestAccount) UnmarshalJSON(input []byte) error {
	bb := &BA{MakerFeeRate: DefaultFeeRate, TakerFeeRate: DefaultFeeRate}
	if err := json.Unmarshal(input, bb); err != nil {
		return err
	}
	*b = BacktestAccount(*bb)
	return nil
}

type BacktestAccountBalanceMap map[string]fixedpoint.Value

func (m BacktestAccountBalanceMap) BalanceMap() types.BalanceMap {
	balances := make(types.BalanceMap)
	for currency, value := range m {
		balances[currency] = types.Balance{
			Currency:  currency,
			Available: value,
			Locked:    fixedpoint.Zero,
		}
	}
	return balances
}

type PersistenceConfig struct {
	Redis *service.RedisPersistenceConfig `json:"redis,omitempty" yaml:"redis,omitempty"`
	Json  *service.JsonPersistenceConfig  `json:"json,omitempty" yaml:"json,omitempty"`
}

type BuildTargetConfig struct {
	Name    string               `json:"name" yaml:"name"`
	Arch    string               `json:"arch" yaml:"arch"`
	OS      string               `json:"os" yaml:"os"`
	LDFlags datatype.StringSlice `json:"ldflags,omitempty" yaml:"ldflags,omitempty"`
	GCFlags datatype.StringSlice `json:"gcflags,omitempty" yaml:"gcflags,omitempty"`
	Imports []string             `json:"imports,omitempty" yaml:"imports,omitempty"`
}

type BuildConfig struct {
	BuildDir string              `json:"buildDir,omitempty" yaml:"buildDir,omitempty"`
	Imports  []string            `json:"imports,omitempty" yaml:"imports,omitempty"`
	Targets  []BuildTargetConfig `json:"targets,omitempty" yaml:"targets,omitempty"`
}

func GetNativeBuildTargetConfig() BuildTargetConfig {
	return BuildTargetConfig{
		Name: "bbgow",
		Arch: runtime.GOARCH,
		OS:   runtime.GOOS,
	}
}

type SyncSymbol struct {
	Symbol  string `json:"symbol" yaml:"symbol"`
	Session string `json:"session" yaml:"session"`
}

func (ss *SyncSymbol) UnmarshalYAML(unmarshal func(a interface{}) error) (err error) {
	var s string
	if err = unmarshal(&s); err == nil {
		aa := strings.SplitN(s, ":", 2)
		if len(aa) > 1 {
			ss.Session = aa[0]
			ss.Symbol = aa[1]
		} else {
			ss.Symbol = aa[0]
		}
		return nil
	}

	type localSyncSymbol SyncSymbol
	var ssNew localSyncSymbol
	if err = unmarshal(&ssNew); err == nil {
		*ss = SyncSymbol(ssNew)
		return nil
	}

	return err
}

func categorizeSyncSymbol(slice []SyncSymbol) (map[string][]string, []string) {
	var rest []string
	var m = make(map[string][]string)
	for _, ss := range slice {
		if len(ss.Session) > 0 {
			m[ss.Session] = append(m[ss.Session], ss.Symbol)
		} else {
			rest = append(rest, ss.Symbol)
		}
	}
	return m, rest
}

type SyncConfig struct {
	// Sessions to sync, if ignored, all defined sessions will sync
	Sessions []string `json:"sessions,omitempty" yaml:"sessions,omitempty"`

	// Symbols is the list of session:symbol pair to sync, if ignored, symbols wlll be discovered by your existing crypto balances
	// Valid formats are: {session}:{symbol},  {symbol} or in YAML object form {symbol: "BTCUSDT", session:"max" }
	Symbols []SyncSymbol `json:"symbols,omitempty" yaml:"symbols,omitempty"`

	// DepositHistory is for syncing deposit history
	DepositHistory bool `json:"depositHistory" yaml:"depositHistory"`

	// WithdrawHistory is for syncing withdraw history
	WithdrawHistory bool `json:"withdrawHistory" yaml:"withdrawHistory"`

	// RewardHistory is for syncing reward history
	RewardHistory bool `json:"rewardHistory" yaml:"rewardHistory"`

	// MarginHistory is for syncing margin related history: loans, repays, interests and liquidations
	MarginHistory bool `json:"marginHistory" yaml:"marginHistory"`

	MarginAssets []string `json:"marginAssets" yaml:"marginAssets"`

	// Since is the date where you want to start syncing data
	Since *types.LooseFormatTime `json:"since,omitempty"`

	// UserDataStream is for real-time sync with websocket user data stream
	UserDataStream *struct {
		Trades       bool `json:"trades,omitempty" yaml:"trades,omitempty"`
		FilledOrders bool `json:"filledOrders,omitempty" yaml:"filledOrders,omitempty"`
	} `json:"userDataStream,omitempty" yaml:"userDataStream,omitempty"`
}

type GoogleSpreadSheetServiceConfig struct {
	JsonTokenFile string `json:"jsonTokenFile" yaml:"jsonTokenFile"`
	SpreadSheetID string `json:"spreadSheetId" yaml:"spreadSheetId"`
}

type ServiceConfig struct {
	GoogleSpreadSheetService *GoogleSpreadSheetServiceConfig `json:"googleSpreadSheet" yaml:"googleSpreadSheet"`
}

type DatabaseConfig struct {
	Driver string `json:"driver" yaml:"driver"`
	DSN    string `json:"dsn" yaml:"dsn"`

	ExtraMigrationPackages []string `json:"extraMigrationPackages" yaml:"extraMigrationPackages"`
}

type EnvironmentConfig struct {
	DisableDefaultKLineSubscription bool `json:"disableDefaultKLineSubscription"`
	DisableHistoryKLinePreload      bool `json:"disableHistoryKLinePreload"`

	// DisableStartUpBalanceQuery disables the balance query in the startup process
	// which initializes the session.Account with the QueryAccount method.
	DisableStartupBalanceQuery bool `json:"disableStartupBalanceQuery"`

	DisableSessionTradeBuffer bool `json:"disableSessionTradeBuffer"`

	DisableMarketDataStore bool `json:"disableMarketDataStore"`

	MaxSessionTradeBufferSize int `json:"maxSessionTradeBufferSize"`
}

type Config struct {
	Build *BuildConfig `json:"build,omitempty" yaml:"build,omitempty"`

	// Imports is deprecated
	// Deprecated: use BuildConfig instead
	Imports []string `json:"imports,omitempty" yaml:"imports,omitempty"`

	Backtest *Backtest `json:"backtest,omitempty" yaml:"backtest,omitempty"`

	Sync *SyncConfig `json:"sync,omitempty" yaml:"sync,omitempty"`

	Notifications *NotificationConfig `json:"notifications,omitempty" yaml:"notifications,omitempty"`

	Persistence *PersistenceConfig `json:"persistence,omitempty" yaml:"persistence,omitempty"`

	Service *ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`

	DatabaseConfig *DatabaseConfig `json:"database,omitempty" yaml:"database,omitempty"`

	Environment *EnvironmentConfig `json:"environment,omitempty" yaml:"environment,omitempty"`

	Sessions map[string]*ExchangeSession `json:"sessions,omitempty" yaml:"sessions,omitempty"`

	RiskControls *RiskControls `json:"riskControls,omitempty" yaml:"riskControls,omitempty"`

	Logging *LoggingConfig `json:"logging,omitempty"`

	ExchangeStrategies      []ExchangeStrategyMount `json:"-" yaml:"-"`
	CrossExchangeStrategies []CrossExchangeStrategy `json:"-" yaml:"-"`

	PnLReporters []PnLReporterConfig `json:"reportPnL,omitempty" yaml:"reportPnL,omitempty"`
}

func (c *Config) Map() (map[string]interface{}, error) {
	text, err := json.Marshal(c)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	err = json.Unmarshal(text, &data)
	if err != nil {
		return nil, err
	}

	// convert strategy config back to the DSL format
	var exchangeStrategies []map[string]interface{}
	for _, m := range c.ExchangeStrategies {
		params, err := m.Map()
		if err != nil {
			return nil, err
		}

		exchangeStrategies = append(exchangeStrategies, params)
	}

	if len(exchangeStrategies) > 0 {
		data["exchangeStrategies"] = exchangeStrategies
	}

	var crossExchangeStrategies []map[string]interface{}
	for _, st := range c.CrossExchangeStrategies {
		strategyID := st.ID()

		var params Stash

		out, err := json.Marshal(st)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(out, &params); err != nil {
			return nil, err
		}

		crossExchangeStrategies = append(crossExchangeStrategies, map[string]interface{}{
			strategyID: params,
		})
	}

	if len(crossExchangeStrategies) > 0 {
		data["crossExchangeStrategies"] = crossExchangeStrategies
	}

	return data, err
}

func (c *Config) YAML() ([]byte, error) {
	m, err := c.Map()
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var enc = yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	err = enc.Encode(m)
	return buf.Bytes(), err
}

func (c *Config) GetSignature() string {
	var s string

	var ps []string

	// for single exchange strategy
	if len(c.ExchangeStrategies) == 1 && len(c.CrossExchangeStrategies) == 0 {
		mount := c.ExchangeStrategies[0].Mounts[0]
		ps = append(ps, mount)

		strategy := c.ExchangeStrategies[0].Strategy

		id := strategy.ID()
		ps = append(ps, id)

		if symbol, ok := dynamic.LookupSymbolField(reflect.ValueOf(strategy)); ok {
			ps = append(ps, symbol)
		}
	}

	startTime := c.Backtest.StartTime.Time()
	ps = append(ps, startTime.Format("2006-01-02"))

	if c.Backtest.EndTime != nil {
		endTime := c.Backtest.EndTime.Time()
		ps = append(ps, endTime.Format("2006-01-02"))
	}

	s = strings.Join(ps, "_")
	return s
}

type Stash map[string]interface{}

func loadStash(config []byte) (Stash, error) {
	stash := make(Stash)
	if err := yaml.Unmarshal(config, stash); err != nil {
		return nil, err
	}

	return stash, nil
}

func LoadBuildConfig(configFile string) (*Config, error) {
	var config Config

	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	// for backward compatible
	if config.Build == nil {
		if len(config.Imports) > 0 {
			config.Build = &BuildConfig{
				BuildDir: "build",
				Imports:  config.Imports,
				Targets: []BuildTargetConfig{
					{Name: "bbgow-amd64-darwin", Arch: "amd64", OS: "darwin"},
					{Name: "bbgow-amd64-linux", Arch: "amd64", OS: "linux"},
				},
			}
		}
	}

	return &config, nil
}

// Load parses the config
func Load(configFile string, loadStrategies bool) (*Config, error) {
	var config Config

	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}

	// for backward compatible
	if config.Build == nil {
		config.Build = &BuildConfig{
			BuildDir: "build",
			Imports:  config.Imports,
			Targets: []BuildTargetConfig{
				{Name: "bbgow-amd64-darwin", Arch: "amd64", OS: "darwin"},
				{Name: "bbgow-amd64-linux", Arch: "amd64", OS: "linux"},
			},
		}
	}

	stash, err := loadStash(content)
	if err != nil {
		return nil, err
	}

	if loadStrategies {
		if err := loadExchangeStrategies(&config, stash); err != nil {
			return nil, err
		}

		if err := loadCrossExchangeStrategies(&config, stash); err != nil {
			return nil, err
		}
	}

	return &config, nil
}

func loadCrossExchangeStrategies(config *Config, stash Stash) (err error) {
	exchangeStrategiesConf, ok := stash["crossExchangeStrategies"]
	if !ok {
		return nil
	}

	if len(LoadedCrossExchangeStrategies) == 0 {
		return errors.New("no cross exchange strategy is registered")
	}

	configList, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return errors.New("expecting list in crossExchangeStrategies")
	}

	for _, entry := range configList {
		configStash, ok := entry.(Stash)
		if !ok {
			return fmt.Errorf("strategy config should be a map, given: %T %+v", entry, entry)
		}

		for id, conf := range configStash {
			// look up the real struct type
			if st, ok := LoadedCrossExchangeStrategies[id]; ok {
				val, err := reUnmarshal(conf, st)
				if err != nil {
					return err
				}

				config.CrossExchangeStrategies = append(config.CrossExchangeStrategies, val.(CrossExchangeStrategy))
			}
		}
	}

	return nil
}

func NewStrategyFromMap(id string, conf interface{}) (SingleExchangeStrategy, error) {
	if st, ok := LoadedExchangeStrategies[id]; ok {
		val, err := reUnmarshal(conf, st)
		if err != nil {
			return nil, err
		}
		return val.(SingleExchangeStrategy), nil
	}

	return nil, fmt.Errorf("strategy %s not found", id)
}

func loadExchangeStrategies(config *Config, stash Stash) (err error) {
	exchangeStrategiesConf, ok := stash["exchangeStrategies"]
	if !ok {
		exchangeStrategiesConf, ok = stash["strategies"]
		if !ok {
			return nil
		}
	}

	if len(LoadedExchangeStrategies) == 0 {
		return errors.New("no exchange strategy is registered")
	}

	configList, ok := exchangeStrategiesConf.([]interface{})
	if !ok {
		return errors.New("expecting list in exchangeStrategies")
	}

	for _, entry := range configList {
		configStash, ok := entry.(Stash)
		if !ok {
			return fmt.Errorf("strategy config should be a map, given: %T %+v", entry, entry)
		}

		var mounts []string
		if val, ok := configStash["on"]; ok {
			switch tv := val.(type) {

			case []string:
				mounts = append(mounts, tv...)

			case string:
				mounts = append(mounts, tv)

			case []interface{}:
				for _, f := range tv {
					s, ok := f.(string)
					if !ok {
						return fmt.Errorf("%+v (%T) is not a string", f, f)
					}

					mounts = append(mounts, s)
				}

			default:
				return fmt.Errorf("unexpected mount type: %T value: %+v", val, val)
			}
		}
		for id, conf := range configStash {

			// look up the real struct type
			if _, ok := LoadedExchangeStrategies[id]; ok {
				st, err := NewStrategyFromMap(id, conf)
				if err != nil {
					return err
				}

				config.ExchangeStrategies = append(config.ExchangeStrategies, ExchangeStrategyMount{
					Mounts:   mounts,
					Strategy: st,
				})
			} else if id != "on" && id != "off" {
				// Show error when we didn't find the Strategy
				return fmt.Errorf("strategy %s in config not found", id)
			}
		}
	}

	return nil
}

func reUnmarshal(conf interface{}, tpe interface{}) (interface{}, error) {
	// get the type "*Strategy"
	rt := reflect.TypeOf(tpe)

	// allocate new object from the given type
	val := reflect.New(rt)

	// now we have &(*Strategy) -> **Strategy
	valRef := val.Interface()

	plain, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(plain, valRef); err != nil {
		return nil, errors.Wrapf(err, "json parsing error, given payload: %s", plain)
	}

	return val.Elem().Interface(), nil
}
