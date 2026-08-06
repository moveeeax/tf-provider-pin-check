package scanner

import "testing"

func TestPinnedConstraint(t *testing.T) {
	cases := []struct {
		raw    string
		strict bool
		want   bool
	}{
		{"1.2.3", false, true},
		{"= 1.2.3", false, true},
		{"=1.2.3", false, true},
		{"~> 5.0", false, true},
		{"~> 5.0", true, false},
		{">= 1.0.0", false, false},
		{"> 1.0.0", false, false},
		{">= 1.0.0, < 2.0.0", false, false},
		{"latest", false, false},
		{"*", false, false},
		{"", false, false},
		{"   ", false, false},
		{">= 1.0.0, = 1.2.3", false, true},
	}
	for _, c := range cases {
		if got := pinnedConstraint(c.raw, c.strict); got != c.want {
			t.Errorf("pinnedConstraint(%q, strict=%v) = %v, want %v", c.raw, c.strict, got, c.want)
		}
	}
}
