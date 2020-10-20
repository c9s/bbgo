package config

import (
	"testing"

	"github.com/stretchr/testify/assert"

	// register the strategies
	_ "github.com/c9s/bbgo/pkg/strategy/buyandhold"
)

func TestLoadStrategies(t *testing.T) {
	type args struct {
		configFile string
	}

	tests := []struct {
		name    string
		args    args
		wantErr bool
		length  int
	}{
		{
			name: "simple",
			args: args{
				configFile: "testdata/strategy.yaml",
			},
			wantErr: false,
			length:  1,
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
			assert.Len(t, config.ExchangeStrategies, tt.length)
		})
	}
}
