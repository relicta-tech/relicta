package policy

import "testing"

func TestCompareIntEqual(t *testing.T) {
	tests := []struct {
		name string
		a    int
		b    any
		want bool
	}{
		{"int_equal", 5, 5, true},
		{"int_not_equal", 5, 6, false},
		{"int_vs_int64_equal", 5, int64(5), true},
		{"int_vs_int64_not_equal", 5, int64(6), false},
		{"int_vs_float64_equal", 5, float64(5), true},
		{"int_vs_float64_not_equal", 5, float64(5.1), false},
		{"int_vs_string", 5, "5", false},
		{"int_vs_bool", 5, true, false},
		{"int_vs_nil", 5, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareIntEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("compareIntEqual(%d, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestCompareFloatEqual(t *testing.T) {
	tests := []struct {
		name string
		a    float64
		b    any
		want bool
	}{
		{"float_equal", 3.14, 3.14, true},
		{"float_not_equal", 3.14, 3.15, false},
		{"float_vs_int_equal", 5.0, 5, true},
		{"float_vs_int_not_equal", 5.1, 5, false},
		{"float_vs_int64_equal", 5.0, int64(5), true},
		{"float_vs_int64_not_equal", 5.1, int64(5), false},
		{"float_vs_string", 5.0, "5.0", false},
		{"float_vs_nil", 5.0, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := compareFloatEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("compareFloatEqual(%f, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name   string
		input  any
		want   float64
		wantOK bool
	}{
		{"float64", float64(3.14), 3.14, true},
		{"float32", float32(2.5), 2.5, true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"string", "42", 0, false},
		{"bool", true, 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.wantOK {
				t.Errorf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestCompareNumeric(t *testing.T) {
	tests := []struct {
		name    string
		a, b    any
		cmp     func(float64, float64) bool
		want    bool
		wantErr bool
	}{
		{"greater", 10, 5, func(a, b float64) bool { return a > b }, true, false},
		{"less", 5, 10, func(a, b float64) bool { return a < b }, true, false},
		{"equal_float", 5.0, float64(5), func(a, b float64) bool { return a == b }, true, false},
		{"invalid_a", "abc", 5, func(a, b float64) bool { return a > b }, false, true},
		{"invalid_b", 5, "abc", func(a, b float64) bool { return a > b }, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareNumeric(tt.a, tt.b, tt.cmp)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareNumeric() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("compareNumeric() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b any
		want bool
	}{
		{"string_equal", "hello", "hello", true},
		{"string_not_equal", "hello", "world", false},
		{"int_equal", 42, 42, true},
		{"int64_equal", int64(42), 42, true},
		{"float64_equal", float64(42), 42, true},
		{"bool_equal", true, true, true},
		{"bool_not_equal", true, false, false},
		{"sprintf_fallback", struct{}{}, struct{}{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := valuesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
