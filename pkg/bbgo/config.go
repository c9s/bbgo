package bbgo

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"runtime"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/datatype"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"
)

type PnLReporterConfig struct {
	AverageCostBySymbols datatype.StringSlice `json:"averageCostBySymbols" yaml:"averageCostBySymbols"`
	Of                   datatype.StringSlice `json:"of" yaml:"of"`
	When                 datatype.StringSlice `json:"when" yaml:"when"`
}

// ExchangeStrategyMount wraps the SingleExchangeStrategy with the ExchangeSession name for mounting
type ExchangeStrategyMount struct {
	// Mounts contains the ExchangeSession name to mount
	Mounts []string

	// Strategy is the strategy we loaded from config
	Strategy SingleExchangeStrategy
}

type SlackNotification struct {
	DefaultChannel string `json:"defaultChannel,omitempty"  yaml:"defaultChannel,omitempty"`
	ErrorChannel   string `json:"errorChannel,omitempty"  yaml:"errorChannel,omitempty"`
}

type NotificationRouting struct {
	Trade       string `json:"trade,omitempty" yaml:"trade,omitempty"`
	Order       string `json:"order,omitempty" yaml:"order,omitempty"`
	SubmitOrder string `json:"submitOrder,omitempty" yaml:"submitOrder,omitempty"`
	PnL         string `json:"pnL,omitempty" yaml:"pnL,omitempty"`
}

type NotificationConfig struct {
	Slack *SlackNotification `json:"slack,omitempty" yaml:"slack,omitempty"`

	SymbolChannels  map[string]string `json:"symbolChannels,omitempty" yaml:"symbolChannels,omitempty"`
	SessionChannels map[string]string `json:"sessionChannels,omitempty" yaml:"sessionChannels,omitempty"`

	Routing *NotificationRouting `json:"routing,omitempty" yaml:"routing,omitempty"`
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

type Backtest struct {
	StartTime string `json:"startTime" yaml:"startTime"`
	EndTime   string `json:"endTime" yaml:"endTime"`

	Account BacktestAccount `json:"account" yaml:"account"`
	Symbols []string        `json:"symbols" yaml:"symbols"`
}

func (t Backtest) ParseEndTime() (time.Time, error) {
	if len(t.EndTime) == 0 {
		return time.Time{}, errors.New("backtest.endTime must be defined")
	}

	return time.Parse("2006-01-02", t.EndTime)
}

func (t Backtest) ParseStartTime() (time.Time, error) {
	if len(t.StartTime) == 0 {
		return time.Time{}, errors.New("backtest.startTime must be defined")
	}

	return time.Parse("2006-01-02", t.StartTime)
}

type BacktestAccount struct {
	MakerCommission  int                       `json:"makerCommission"`
	TakerCommission  int                       `json:"takerCommission"`
	BuyerCommission  int                       `json:"buyerCommission"`
	SellerCommission int                       `json:"sellerCommission"`
	Balances         BacktestAccountBalanceMap `json:"balances" yaml:"balances"`
}

type BacktestAccountBalanceMap map[string]fixedpoint.Value

func (m BacktestAccountBalanceMap) BalanceMap() types.BalanceMap {
	balances := make(types.BalanceMap)
	for currency, value := range m {
		balances[currency] = types.Balance{
			Currency:  currency,
			Available: value,
			Locked:    0,
		}
	}
	return balances
}

type RedisPersistenceConfig struct {
	Host     string `json:"host" env:"REDIS_HOST"`
	Port     string `json:"port" env:"REDIS_PORT"`
	Password string `json:"password" env:"REDIS_PASSWORD"`
	DB       int    `json:"db" env:"REDIS_DB"`
}

type JsonPersistenceConfig struct {
	Directory string `json:"directory"`
}

type PersistenceConfig struct {
	Redis *RedisPersistenceConfig `json:"redis,omitempty" yaml:"redis,omitempty"`
	Json  *JsonPersistenceConfig  `json:"json,omitempty" yaml:"json,omitempty"`
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

type Config struct {
	Build *BuildConfig `json:"build,omitempty" yaml:"build,omitempty"`

	// Imports is deprecated
	// Deprecated: use BuildConfig instead
	Imports []string `json:"imports,omitempty" yaml:"imports,omitempty"`

	Backtest *Backtest `json:"backtest,omitempty" yaml:"backtest,omitempty"`

	Notifications *NotificationConfig `json:"notifications,omitempty" yaml:"notifications,omitempty"`

	Persistence *PersistenceConfig `json:"persistence,omitempty" yaml:"persistence,omitempty"`

	Sessions map[string]*ExchangeSession `json:"sessions,omitempty" yaml:"sessions,omitempty"`

	RiskControls *RiskControls `json:"riskControls,omitempty" yaml:"riskControls,omitempty"`

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
		strategyID := m.Strategy.ID()

		var params Stash

		out, err := json.Marshal(m.Strategy)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(out, &params); err != nil {
			return nil, err
		}

		exchangeStrategies = append(exchangeStrategies, map[string]interface{}{
			"on":       m.Mounts,
			strategyID: params,
		})
	}
	data["exchangeStrategies"] = exchangeStrategies

	return data, err
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
		return nil
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
			if values, ok := val.([]string); ok {
				mounts = append(mounts, values...)
			} else if str, ok := val.(string); ok {
				mounts = append(mounts, str)
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
