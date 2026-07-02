package binance

import (
	"testing"

	"github.com/c9s/bbgo/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestConvertSubscription_MarkPrice(t *testing.T) {
	cases := []struct {
		name string
		sub  types.Subscription
		want string
	}{
		{
			name: "no interval",
			sub:  types.Subscription{Channel: types.MarkPriceChannel, Symbol: "BTCUSDT"},
			want: "btcusdt@markPrice",
		},
		{
			name: "1s interval",
			sub:  types.Subscription{Channel: types.MarkPriceChannel, Symbol: "BTCUSDT", Options: types.SubscribeOptions{Interval: types.Interval1s}},
			want: "btcusdt@markPrice@1s",
		},
		{
			name: "custom 3s interval string",
			sub:  types.Subscription{Channel: types.MarkPriceChannel, Symbol: "BTCUSDT", Options: types.SubscribeOptions{Interval: types.Interval("3s")}},
			want: "btcusdt@markPrice@3s",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := convertSubscription(c.sub)
			assert.Equal(t, c.want, got)
		})
	}
}
