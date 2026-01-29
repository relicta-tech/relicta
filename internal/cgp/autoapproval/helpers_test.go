package autoapproval

import "testing"

func TestCompareIntEqual(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    any
		want bool
	}{
		{"int_match", 10, 10, true},
		{"int_no_match", 10, 11, false},
		{"int_vs_int64", 10, int64(10), true},
		{"int_vs_int64_no_match", 10, int64(11), false},
		{"int_vs_float64", 10, float64(10), true},
		{"int_vs_float64_no_match", 10, float64(10.5), false},
		{"int_vs_string", 10, "10", false},
		{"int_vs_nil", 10, nil, false},
		{"int_vs_bool", 0, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareIntEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("compareIntEqual(%d, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
