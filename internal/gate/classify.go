// Package gate implements the cdk.out Lambda bundle-size gate.
package gate

// AWS's hard limit on the UNZIPPED deployment package (function code + all
// layers) of a zip-packaged Lambda. This is the exact byte count AWS reports
// in the InvalidRequest error (262144000 == 250 * 1024 * 1024).
const DefaultHardLimitBytes int64 = 262_144_000

// Advisory threshold — above this, the bundle should be trimmed proactively
// (a few more transitive deps and it breaks a deploy). WARN never fails.
const DefaultWarnLimitBytes int64 = 200 * 1024 * 1024

// Limits carries the operator-tunable thresholds.
type Limits struct {
	Hard int64 // strictly above → FAIL
	Warn int64 // strictly above (but within Hard) → WARN
}

func DefaultLimits() Limits {
	return Limits{Hard: DefaultHardLimitBytes, Warn: DefaultWarnLimitBytes}
}

// Classify buckets an unzipped size into FAIL / WARN / OK.
func (l Limits) Classify(size int64) string {
	switch {
	case size > l.Hard:
		return "FAIL"
	case size > l.Warn:
		return "WARN"
	default:
		return "OK"
	}
}
