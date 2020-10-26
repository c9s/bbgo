package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	// register the strategies
	_ "github.com/c9s/bbgo/pkg/strategy/buyandhold"
)

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
			name:    "strategy",
			args:    args{configFile: "testdata/strategy.yaml"},
			wantErr: false,
			f: func(t *testing.T, config *Config) {
				assert.Len(t, config.ExchangeStrategies, 1)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := Load(tt.args.configFile)
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
