package gate

import "testing"

func TestClassifyThresholds(t *testing.T) {
	l := DefaultLimits()
	cases := []struct {
		size int64
		want string
	}{
		{0, "OK"},
		{DefaultWarnLimitBytes, "OK"}, // boundary is inclusive-OK
		{DefaultWarnLimitBytes + 1, "WARN"},
		{DefaultHardLimitBytes, "WARN"}, // exactly at limit still deploys
		{DefaultHardLimitBytes + 1, "FAIL"},
	}
	for _, c := range cases {
		if got := l.Classify(c.size); got != c.want {
			t.Errorf("Classify(%d) = %q, want %q", c.size, got, c.want)
		}
	}
}

func TestClassifyCustomLimits(t *testing.T) {
	l := Limits{Hard: 100, Warn: 50}
	if got := l.Classify(101); got != "FAIL" {
		t.Errorf("custom hard limit: got %q, want FAIL", got)
	}
	if got := l.Classify(51); got != "WARN" {
		t.Errorf("custom warn limit: got %q, want WARN", got)
	}
}
