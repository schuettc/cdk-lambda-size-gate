package gate

import (
	"fmt"
	"sort"
	"strings"
)

const mib = 1024.0 * 1024.0

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// FormatReport renders a per-function size table sorted largest-total-first.
// Functions with unmeasured layers get a "≥" prefix: their true total is a
// lower bound because ARN-referenced layers aren't staged in cdk.out.
func FormatReport(assets []FunctionAsset, limits Limits) string {
	if len(assets) == 0 {
		return "No zip-packaged Lambda assets found in cdk.out (nothing to check)."
	}
	rows := make([]FunctionAsset, len(assets))
	copy(rows, assets)
	sort.Slice(rows, func(i, j int) bool { return rows[i].TotalBytes() > rows[j].TotalBytes() })

	header := fmt.Sprintf("%-6s  %11s  %10s  %10s  %-22s  %-40s  %s",
		"STATUS", "TOTAL (MiB)", "FN (MiB)", "LYR (MiB)", "STACK", "FUNCTION", "ASSET")
	lines := []string{header, strings.Repeat("-", len(header))}
	lowerBound := false
	for _, a := range rows {
		mark := " "
		if a.UnmeasuredLayers > 0 {
			mark = "≥"
			lowerBound = true
		}
		lines = append(lines, fmt.Sprintf("%-6s  %s%10.1f  %10.1f  %10.1f  %-22s  %-40s  %s",
			limits.Classify(a.TotalBytes()), mark,
			float64(a.TotalBytes())/mib, float64(a.FunctionBytes)/mib, float64(a.LayerBytes)/mib,
			truncate(a.Stack, 22), truncate(a.LogicalID, 40), truncate(a.AssetHash, 12)))
	}
	lines = append(lines, "")
	if lowerBound {
		lines = append(lines,
			"≥ = function references layer(s) not staged in cdk.out (bare ARN / cross-stack);",
			"    the true total is at least the value shown.", "")
	}
	lines = append(lines, fmt.Sprintf(
		"Thresholds: WARN > %.0f MiB, FAIL > %.0f MiB (AWS hard limit %d bytes unzipped, function + layers).",
		float64(limits.Warn)/mib, float64(limits.Hard)/mib, limits.Hard))
	return strings.Join(lines, "\n")
}

// Annotations returns GitHub Actions workflow-command lines (warnings first,
// then errors) for every WARN/FAIL function.
func Annotations(assets []FunctionAsset, limits Limits) []string {
	var warns, fails []string
	for _, a := range assets {
		switch limits.Classify(a.TotalBytes()) {
		case "WARN":
			warns = append(warns, fmt.Sprintf(
				"::warning::Lambda %s/%s unzipped bundle (fn+layers) is %.1f MiB — approaching the %.0f MiB limit.",
				a.Stack, a.LogicalID, float64(a.TotalBytes())/mib, float64(limits.Hard)/mib))
		case "FAIL":
			fails = append(fails, fmt.Sprintf(
				"::error::Lambda %s/%s unzipped bundle (fn+layers) is %.1f MiB, over the AWS %d byte (%.0f MiB) unzipped limit — deploy would fail.",
				a.Stack, a.LogicalID, float64(a.TotalBytes())/mib, limits.Hard, float64(limits.Hard)/mib))
		}
	}
	return append(warns, fails...)
}
