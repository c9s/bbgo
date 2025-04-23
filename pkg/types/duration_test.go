package types

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestParseSimpleDuration(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name    string
		args    args
		want    *SimpleDuration
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "3h",
			args: args{
				s: "3h",
			},
			want:    &SimpleDuration{Num: 3, Unit: "h", Duration: Duration(3 * time.Hour)},
			wantErr: assert.NoError,
		},
		{
			name: "3d",
			args: args{
				s: "3d",
			},
			want:    &SimpleDuration{Num: 3, Unit: "d", Duration: Duration(3 * 24 * time.Hour)},
			wantErr: assert.NoError,
		},
		{
			name: "3w",
			args: args{
				s: "3w",
			},
			want:    &SimpleDuration{Num: 3, Unit: "w", Duration: Duration(3 * 7 * 24 * time.Hour)},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSimpleDuration(tt.args.s)
			if !tt.wantErr(t, err, fmt.Sprintf("ParseSimpleDuration(%v)", tt.args.s)) {
				return
			}
			assert.Equalf(t, tt.want, got, "ParseSimpleDuration(%v)", tt.args.s)
		})
	}
}

func TestSerialization(t *testing.T) {
	d := Duration(3 * time.Minute)
	jsonData, err := json.Marshal(&d)
	assert.NoError(t, err)
	assert.Equal(t, `"3m0s"`, string(jsonData))

	var d2 Duration
	err = json.Unmarshal(jsonData, &d2)
	assert.NoError(t, err)
	assert.Equal(t, d, d2)

	ymalData, err := yaml.Marshal(&d)
	assert.NoError(t, err)
	assert.Equal(t, "3m0s\n", string(ymalData))

	err = yaml.Unmarshal(ymalData, &d2)
	assert.NoError(t, err)
	assert.Equal(t, d, d2)
}
