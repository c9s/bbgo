package bbgo

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func init() {
	RegisterStrategy("test", &TestStrategy{})
}

type TestStrategy struct {
	Symbol            string  `json:"symbol"`
	Interval          string  `json:"interval"`
	BaseQuantity      float64 `json:"baseQuantity"`
	MaxAssetQuantity  float64 `json:"maxAssetQuantity"`
	MinDropPercentage float64 `json:"minDropPercentage"`
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
				assert.NotNil(t, config.Notifications.SessionChannels)
				assert.NotNil(t, config.Notifications.SymbolChannels)
				assert.Equal(t, map[string]string{
					"^BTC": "#btc",
					"^ETH": "#eth",
				}, config.Notifications.SymbolChannels)
				assert.NotNil(t, config.Notifications.Routing)
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
				assert.NotNil(t, config.Backtest.Account.Balances)
				assert.Len(t, config.Backtest.Account.Balances, 2)
				assert.NotEmpty(t, config.Backtest.StartTime)
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
