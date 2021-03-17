package fixedpoint

import "testing"

func TestParse(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name                 string
		args                 args
		wantNum              int64
		wantNumDecimalPoints int
		wantErr              bool
	}{
		{
			args: args{ input: "-99.9" },
			wantNum: -999,
			wantNumDecimalPoints: 1,
			wantErr: false,
		},
		{
			args: args{ input: "0.12345678" },
			wantNum: 12345678,
			wantNumDecimalPoints: 8,
			wantErr: false,
		},
		{
			args: args{ input: "a" },
			wantNum: 0,
			wantNumDecimalPoints: 0,
			wantErr: true,
		},
		{
			args: args{ input: "0.1" },
			wantNum: 1,
			wantNumDecimalPoints: 1,
			wantErr: false,
		},
		{
			args: args{ input: "100" },
			wantNum: 100,
			wantNumDecimalPoints: 0,
			wantErr: false,
		},
		{
			args: args{ input: "100.9999" },
			wantNum: 1009999,
			wantNumDecimalPoints: 4,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNum, gotNumDecimalPoints, err := Parse(tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNum != tt.wantNum {
				t.Errorf("Parse() gotNum = %v, want %v", gotNum, tt.wantNum)
			}
			if gotNumDecimalPoints != tt.wantNumDecimalPoints {
				t.Errorf("Parse() gotNumDecimalPoints = %v, want %v", gotNumDecimalPoints, tt.wantNumDecimalPoints)
			}
		})
	}
}
