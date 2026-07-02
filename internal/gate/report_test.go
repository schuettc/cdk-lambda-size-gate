package gate

import (
	"strings"
	"testing"
)

func TestFormatReportEmpty(t *testing.T) {
	got := FormatReport(nil, DefaultLimits())
	if !strings.Contains(got, "No zip-packaged Lambda assets found") {
		t.Errorf("empty report missing sentinel: %q", got)
	}
}

func TestFormatReportSortsAndMarksLowerBound(t *testing.T) {
	assets := []FunctionAsset{
		{Stack: "S", LogicalID: "Small", AssetHash: repeat("aa", 32), FunctionBytes: 1 * 1024 * 1024},
		{Stack: "S", LogicalID: "Big", AssetHash: repeat("bb", 32), FunctionBytes: 300 * 1024 * 1024},
		{Stack: "S", LogicalID: "Arny", AssetHash: repeat("cc", 32), FunctionBytes: 2 * 1024 * 1024, UnmeasuredLayers: 1},
	}
	got := FormatReport(assets, DefaultLimits())
	if strings.Index(got, "Big") > strings.Index(got, "Small") {
		t.Error("report not sorted largest-first")
	}
	if !strings.Contains(got, "≥") {
		t.Error("lower-bound marker missing for unmeasured layers")
	}
	if !strings.Contains(got, "FAIL") || !strings.Contains(got, "OK") {
		t.Errorf("statuses missing:\n%s", got)
	}
	if !strings.Contains(got, "Thresholds:") {
		t.Error("thresholds footer missing")
	}
}

func TestAnnotations(t *testing.T) {
	assets := []FunctionAsset{
		{Stack: "S", LogicalID: "Warny", FunctionBytes: DefaultWarnLimitBytes + 1},
		{Stack: "S", LogicalID: "Faily", FunctionBytes: DefaultHardLimitBytes + 1},
		{Stack: "S", LogicalID: "Fine", FunctionBytes: 10},
	}
	got := Annotations(assets, DefaultLimits())
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2: %v", len(got), got)
	}
	if !strings.HasPrefix(got[0], "::warning::") || !strings.Contains(got[0], "Warny") {
		t.Errorf("warning line wrong: %q", got[0])
	}
	if !strings.HasPrefix(got[1], "::error::") || !strings.Contains(got[1], "Faily") {
		t.Errorf("error line wrong: %q", got[1])
	}
}
