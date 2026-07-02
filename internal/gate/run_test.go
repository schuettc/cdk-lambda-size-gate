package gate

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func runGate(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestRunFailsOversized(t *testing.T) {
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack: "S", LogicalID: "Fat", AssetHash: repeat("ab", 32),
		AssetBytes: DefaultHardLimitBytes + 1,
	})
	code, stdout, stderr := runGate(t, "-cdk-out", cdkOut)
	if code != 1 {
		t.Fatalf("exit = %d, want 1", code)
	}
	if !strings.Contains(stdout, "::error::") {
		t.Error("missing ::error:: annotation")
	}
	if !strings.Contains(stderr, "Bundle-size gate FAILED") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunPassesSmall(t *testing.T) {
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack: "S", LogicalID: "Slim", AssetHash: repeat("cd", 32), AssetBytes: 1024,
	})
	code, stdout, _ := runGate(t, "-cdk-out", cdkOut)
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Bundle-size gate passed.") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestRunMissingCdkOut(t *testing.T) {
	code, _, stderr := runGate(t, "-cdk-out", filepath.Join(t.TempDir(), "nope"))
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if !strings.Contains(stderr, "Run `cdk synth` first") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestRunCustomLimits(t *testing.T) {
	// Knobs, not constants: a 2000-byte fn fails a -hard-limit 1000 gate.
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack: "S", LogicalID: "Tiny", AssetHash: repeat("ef", 32), AssetBytes: 2000,
	})
	code, _, _ := runGate(t, "-cdk-out", cdkOut, "-hard-limit", "1000", "-warn-limit", "500")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 with custom limits", code)
	}
}

func TestRunBadFlags(t *testing.T) {
	code, _, _ := runGate(t, "-definitely-not-a-flag")
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
