package xmaker

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/types"

	. "github.com/c9s/bbgo/pkg/testing/testhelper"
)

func TestDepthRatioSignal_CalculateSignal(t *testing.T) {
	type fields struct {
		PriceRange fixedpoint.Value
		MinRatio   float64
		symbol     string
		book       *types.StreamOrderBook
	}
	type args struct {
		ctx        context.Context
		bids, asks types.PriceVolumeSlice
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    float64
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name: "medium short",
			fields: fields{
				PriceRange: fixedpoint.NewFromFloat(0.02),
				MinRatio:   0.01,
				symbol:     "BTCUSDT",
			},
			args: args{
				ctx: context.Background(),
				asks: PriceVolumeSliceFromText(`
					19310,1.0
					19320,0.2
					19330,0.3
					19340,0.4
					19350,0.5
				`),
				bids: PriceVolumeSliceFromText(`
					19300,0.1
					19290,0.2
					19280,0.3
					19270,0.4
					19260,0.5
				`),
			},
			want:    -0.4641,
			wantErr: assert.NoError,
		},
		{
			name: "strong short",
			fields: fields{
				PriceRange: fixedpoint.NewFromFloat(0.02),
				MinRatio:   0.01,
				symbol:     "BTCUSDT",
			},
			args: args{
				ctx: context.Background(),
				asks: PriceVolumeSliceFromText(`
					19310,10.0
					19320,0.2
					19330,0.3
					19340,0.4
					19350,0.5
				`),
				bids: PriceVolumeSliceFromText(`
					19300,0.1
					19290,0.1
					19280,0.1
					19270,0.1
					19260,0.1
				`),
			},
			want:    -1.8322,
			wantErr: assert.NoError,
		},
		{
			name: "strong long",
			fields: fields{
				PriceRange: fixedpoint.NewFromFloat(0.02),
				MinRatio:   0.01,
				symbol:     "BTCUSDT",
			},
			args: args{
				ctx: context.Background(),
				asks: PriceVolumeSliceFromText(`
					19310,0.1
					19320,0.1
					19330,0.1
					19340,0.1
					19350,0.1
				`),
				bids: PriceVolumeSliceFromText(`
					19300,10.0
					19290,0.1
					19280,0.1
					19270,0.1
					19260,0.1
				`),
			},
			want:    1.81623,
			wantErr: assert.NoError,
		},
		{
			name: "normal",
			fields: fields{
				PriceRange: fixedpoint.NewFromFloat(0.02),
				MinRatio:   0.01,
				symbol:     "BTCUSDT",
			},
			args: args{
				ctx: context.Background(),
				asks: PriceVolumeSliceFromText(`
					19310,0.1
					19320,0.2
					19330,0.3
					19340,0.4
					19350,0.5
				`),
				bids: PriceVolumeSliceFromText(`
					19300,0.1
					19290,0.2
					19280,0.3
					19270,0.4
					19260,0.5
				`),
			},
			want:    0,
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &DepthRatioSignal{
				PriceRange: tt.fields.PriceRange,
				MinRatio:   tt.fields.MinRatio,
				symbol:     tt.fields.symbol,
				book:       types.NewStreamBook("BTCUSDT", types.ExchangeBinance),
			}

			s.book.Load(types.SliceOrderBook{
				Symbol: "BTCUSDT",
				Bids:   tt.args.bids,
				Asks:   tt.args.asks,
			})

			got, err := s.CalculateSignal(tt.args.ctx)
			if !tt.wantErr(t, err, fmt.Sprintf("CalculateSignal(%v)", tt.args.ctx)) {
				return
			}

			assert.InDeltaf(t, tt.want, got, 0.001, "CalculateSignal(%v)", tt.args.ctx)
		})
	}
}
