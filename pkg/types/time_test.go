package types

import (
	"testing"
	"time"
)

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
			if err := tt.t.UnmarshalJSON(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
