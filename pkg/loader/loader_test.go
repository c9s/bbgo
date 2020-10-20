package loader

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
			strategies, err := LoadExchangeStrategies(tt.args.configFile)
			if err != nil {
				t.Errorf("LoadExchangeStrategies() error = %v", err)
			} else {
				if tt.wantErr {
					t.Errorf("LoadExchangeStrategies() error = %v, wantErr %v", err, tt.wantErr)
				}
			}

			t.Logf("%+v", strategies[0])

			assert.Len(t, strategies, tt.length)
		})
	}
}
