package bbgo

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

func init() {
	RegisterStrategy("test", &TestStrategy{})
}

type TestStrategy struct {
	Symbol            string           `json:"symbol"`
	Interval          string           `json:"interval"`
	BaseQuantity      fixedpoint.Value `json:"baseQuantity"`
	MaxAssetQuantity  fixedpoint.Value `json:"maxAssetQuantity"`
	MinDropPercentage fixedpoint.Value `json:"minDropPercentage"`
}

func (s *TestStrategy) ID() string {
	return "test"
}

func (s *TestStrategy) Run(ctx context.Context, orderExecutor OrderExecutor, session *ExchangeSession) error {
	return nil
}

func TestLoadConfig(t *testing.T) {
	type args struct {
		configFile string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
		f       func(t *testing.T, config *Config)
	}{
		{
			name:    "notification",
			args:    args{configFile: "testdata/notification.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.NotNil(t, config.Notifications)
				assert.Equal(t, "#dev-bbgo", config.Notifications.Slack.DefaultChannel)
				assert.Equal(t, "#error", config.Notifications.Slack.ErrorChannel)
			},
		},

		{
			name:    "strategy",
			args:    args{configFile: "testdata/strategy.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.Len(t, config.ExchangeStrategies, 1)
				assert.Equal(t, []ExchangeStrategyMount{{
					Mounts: []string{"binance"},
					Strategy: &TestStrategy{
						Symbol:            "BTCUSDT",
						Interval:          "1m",
						BaseQuantity:      fixedpoint.NewFromFloat(0.1),
						MaxAssetQuantity:  fixedpoint.NewFromFloat(1.1),
						MinDropPercentage: fixedpoint.NewFromFloat(-0.05),
					},
				}}, config.ExchangeStrategies)

				m, err := config.Map()
				assert.NoError(t, err)
				assert.Equal(t, map[string]interface{}{
					"sessions": map[string]interface{}{
						"max": map[string]interface{}{
							"exchange":                "max",
							"envVarPrefix":            "MAX",
							"takerFeeRate":            0.,
							"makerFeeRate":            0.,
							"modifyOrderAmountForFee": false,
						},
						"binance": map[string]interface{}{
							"exchange":                "binance",
							"envVarPrefix":            "BINANCE",
							"takerFeeRate":            0.,
							"makerFeeRate":            0.,
							"modifyOrderAmountForFee": false,
						},
						"ftx": map[string]interface{}{
							"exchange":                "ftx",
							"envVarPrefix":            "FTX",
							"takerFeeRate":            0.,
							"makerFeeRate":            0.,
							"modifyOrderAmountForFee": true,
						},
					},
					"build": map[string]interface{}{
						"buildDir": "build",
						"targets": []interface{}{
							map[string]interface{}{
								"name": "bbgow-amd64-darwin",
								"arch": "amd64",
								"os":   "darwin",
							},
							map[string]interface{}{
								"name": "bbgow-amd64-linux",
								"arch": "amd64",
								"os":   "linux",
							},
						},
					},
					"exchangeStrategies": []map[string]interface{}{
						{
							"on": []string{"binance"},
							"test": map[string]interface{}{
								"symbol":            "BTCUSDT",
								"baseQuantity":      0.1,
								"interval":          "1m",
								"maxAssetQuantity":  1.1,
								"minDropPercentage": -0.05,
							},
						},
					},
				}, m)

				yamlText, err := config.YAML()
				assert.NoError(t, err)

				yamlTextSource, err := ioutil.ReadFile("testdata/strategy.yaml")
				assert.NoError(t, err)

				var sourceMap map[string]interface{}
				err = yaml.Unmarshal(yamlTextSource, &sourceMap)
				assert.NoError(t, err)
				delete(sourceMap, "build")

				var actualMap map[string]interface{}
				err = yaml.Unmarshal(yamlText, &actualMap)
				assert.NoError(t, err)
				delete(actualMap, "build")

				assert.Equal(t, sourceMap, actualMap)
			},
		},

		{
			name:    "persistence",
			args:    args{configFile: "testdata/persistence.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.NotNil(t, config.Persistence)
				assert.NotNil(t, config.Persistence.Redis)
				assert.NotNil(t, config.Persistence.Json)
			},
		},

		{
			name:    "order_executor",
			args:    args{configFile: "testdata/order_executor.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.Len(t, config.Sessions, 2)

				session, ok := config.Sessions["max"]
				assert.True(t, ok)
				assert.NotNil(t, session)

				riskControls := config.RiskControls
				assert.NotNil(t, riskControls)
				assert.NotNil(t, riskControls.SessionBasedRiskControl)

				conf, ok := riskControls.SessionBasedRiskControl["max"]
				assert.True(t, ok)
				assert.NotNil(t, conf)
				assert.NotNil(t, conf.OrderExecutor)
				assert.NotNil(t, conf.OrderExecutor.BySymbol)

				executorConf, ok := conf.OrderExecutor.BySymbol["BTCUSDT"]
				assert.True(t, ok)
				assert.NotNil(t, executorConf)
			},
		},
		{
			name:    "backtest",
			args:    args{configFile: "testdata/backtest.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.Len(t, config.ExchangeStrategies, 1)
				assert.NotNil(t, config.Backtest)
				assert.NotNil(t, config.Backtest.Account)
				assert.NotNil(t, config.Backtest.Account["binance"].Balances)
				assert.Len(t, config.Backtest.Account["binance"].Balances, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := Load(tt.args.configFile, true)
			if err != nil {
				t.Errorf("Load() error = %v", err)
				return
			} else {
				if tt.wantErr {
					t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			}

			assert.NotNil(t, config)

			if tt.f != nil {
				tt.f(t, config)
			}
		})
	}
}

func TestSyncSymbol(t *testing.T) {
	t.Run("symbol", func(t *testing.T) {
		var ss []SyncSymbol
		var err = yaml.Unmarshal([]byte(`- BTCUSDT`), &ss)
		assert.NoError(t, err)
		assert.Equal(t, []SyncSymbol{
			{Symbol: "BTCUSDT"},
		}, ss)
	})

	t.Run("session:symbol", func(t *testing.T) {
		var ss []SyncSymbol
		var err = yaml.Unmarshal([]byte(`- max:BTCUSDT`), &ss)
		assert.NoError(t, err)
		assert.Equal(t, []SyncSymbol{
			{Session: "max", Symbol: "BTCUSDT"},
		}, ss)
	})

	t.Run("object", func(t *testing.T) {
		var ss []SyncSymbol
		var err = yaml.Unmarshal([]byte(`- { session: "max", symbol: "BTCUSDT" }`), &ss)
		assert.NoError(t, err)
		assert.Equal(t, []SyncSymbol{
			{Session: "max", Symbol: "BTCUSDT"},
		}, ss)
	})
}

func TestBackTestFeeMode(t *testing.T) {
	var mode BacktestFeeMode
	var err = yaml.Unmarshal([]byte(`quote`), &mode)
	assert.NoError(t, err)
	assert.Equal(t, BacktestFeeModeQuote, mode)
}

func Test_categorizeSyncSymbol(t *testing.T) {
	var ss []SyncSymbol
	var err = yaml.Unmarshal([]byte(`
- BTCUSDT
- ETHUSDT
- max:MAXUSDT
- max:USDTTWD
- binance:BNBUSDT
`), &ss)
	assert.NoError(t, err)
	assert.NotEmpty(t, ss)

	sm, rest := categorizeSyncSymbol(ss)
	assert.NotEmpty(t, rest)
	assert.NotEmpty(t, sm)
	assert.Equal(t, []string{"MAXUSDT", "USDTTWD"}, sm["max"])
	assert.Equal(t, []string{"BNBUSDT"}, sm["binance"])
}
