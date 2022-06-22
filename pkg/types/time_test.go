package types

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseLooseFormatTime_alias_now(t *testing.T) {
	lt, err := ParseLooseFormatTime("now")
	assert.NoError(t, err)

	now := time.Now()
	assert.True(t, now.Sub(lt.Time()) < 10*time.Millisecond)
}

func TestParseLooseFormatTime_alias_yesterday(t *testing.T) {
	lt, err := ParseLooseFormatTime("yesterday")
	assert.NoError(t, err)

	tt := time.Now().AddDate(0, 0, -1)
	assert.True(t, tt.Sub(lt.Time()) < 10*time.Millisecond)
}

func TestLooseFormatTime_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		t       LooseFormatTime
		args    []byte
		wantErr bool
	}{
		{
			name: "simple date",
			args: []byte("\"2021-01-01\""),
			t:    LooseFormatTime(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		{
			name: "utc",
			args: []byte("\"2021-01-01T12:10:10\""),
			t:    LooseFormatTime(time.Date(2021, 1, 1, 12, 10, 10, 0, time.UTC)),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v LooseFormatTime
			if err := v.UnmarshalJSON(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				assert.Equal(t, v.Time(), tt.t.Time())
			}
		})
	}
}

func TestMillisecondTimestamp_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		t       MillisecondTimestamp
		args    []byte
		wantErr bool
	}{
		{
			name: "millisecond in string",
			args: []byte("\"1620289117764\""),
			t:    MillisecondTimestamp(time.Unix(0, 1620289117764*int64(time.Millisecond))),
		},
		{
			name: "millisecond in number",
			args: []byte("1620289117764"),
			t:    MillisecondTimestamp(time.Unix(0, 1620289117764*int64(time.Millisecond))),
		},
		{
			name: "millisecond in decimal",
			args: []byte("1620289117.764"),
			t:    MillisecondTimestamp(time.Unix(0, 1620289117764*int64(time.Millisecond))),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v MillisecondTimestamp
			if err := v.UnmarshalJSON(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				assert.Equal(t, tt.t.Time(), v.Time())
			}
		})
	}
}
