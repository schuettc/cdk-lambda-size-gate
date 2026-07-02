package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixtureSpec drives makeCdkOut. Layers fields are used from Task 5 on.
type fixtureSpec struct {
	Stack       string
	LogicalID   string
	AssetHash   string
	AssetBytes  int64
	PackageType string   // "" => Zip
	LayerRefs   []string // logical IDs of LayerVersion resources to attach via Ref
	LayerARNs   []string // raw ARN strings to attach (unmeasurable)
	Layers      map[string]layerSpec
}

type layerSpec struct {
	AssetHash  string
	AssetBytes int64
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// makeCdkOut materializes a minimal cdk.out: template + assets manifest +
// sparse staged asset dirs. Direct port of the Python test helper.
func makeCdkOut(t *testing.T, dir string, spec fixtureSpec) string {
	t.Helper()
	cdkOut := filepath.Join(dir, "cdk.out")
	if err := os.MkdirAll(cdkOut, 0o755); err != nil {
		t.Fatal(err)
	}

	pkgType := spec.PackageType
	if pkgType == "" {
		pkgType = "Zip"
	}
	var code map[string]any
	if pkgType == "Image" {
		code = map[string]any{"ImageUri": "123.dkr.ecr.us-east-1.amazonaws.com/repo:tag"}
	} else {
		code = map[string]any{"S3Bucket": "cdk-assets-bucket", "S3Key": spec.AssetHash + ".zip"}
	}

	var layers []any
	for _, ref := range spec.LayerRefs {
		layers = append(layers, map[string]any{"Ref": ref})
	}
	for _, arn := range spec.LayerARNs {
		layers = append(layers, arn)
	}
	fnProps := map[string]any{"PackageType": pkgType, "Code": code}
	if layers != nil {
		fnProps["Layers"] = layers
	}

	resources := map[string]any{
		spec.LogicalID: map[string]any{
			"Type":       "AWS::Lambda::Function",
			"Properties": fnProps,
		},
	}
	files := map[string]any{}
	if pkgType != "Image" {
		files[spec.AssetHash] = map[string]any{
			"source": map[string]any{"path": "asset." + spec.AssetHash, "packaging": "zip"},
		}
		writeSparse(t, filepath.Join(cdkOut, "asset."+spec.AssetHash, "handler.py"), spec.AssetBytes)
	}
	for id, l := range spec.Layers {
		resources[id] = map[string]any{
			"Type": "AWS::Lambda::LayerVersion",
			"Properties": map[string]any{
				"Content": map[string]any{"S3Bucket": "cdk-assets-bucket", "S3Key": l.AssetHash + ".zip"},
			},
		}
		files[l.AssetHash] = map[string]any{
			"source": map[string]any{"path": "asset." + l.AssetHash, "packaging": "zip"},
		}
		writeSparse(t, filepath.Join(cdkOut, "asset."+l.AssetHash, "layer.bin"), l.AssetBytes)
	}

	writeJSON(t, filepath.Join(cdkOut, spec.Stack+".template.json"),
		map[string]any{"Resources": resources})
	writeJSON(t, filepath.Join(cdkOut, spec.Stack+".assets.json"),
		map[string]any{"files": files})
	return cdkOut
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func TestGateFailsOnOversizedZipLambda(t *testing.T) {
	// The regression guard: an over-limit zip Lambda must FAIL the gate.
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack:      "BhRefresh-dev",
		LogicalID:  "ResultsReconcileFunctionAB96B78D",
		AssetHash:  repeat("deadbeef", 8),
		AssetBytes: DefaultHardLimitBytes + 1,
	})
	assets, anyFail := Evaluate(cdkOut, DefaultLimits())
	if !anyFail {
		t.Fatal("anyFail = false, want true")
	}
	if len(assets) != 1 {
		t.Fatalf("len(assets) = %d, want 1", len(assets))
	}
	if got := DefaultLimits().Classify(assets[0].TotalBytes()); got != "FAIL" {
		t.Errorf("status = %q, want FAIL", got)
	}
	if assets[0].LogicalID != "ResultsReconcileFunctionAB96B78D" {
		t.Errorf("logical id = %q", assets[0].LogicalID)
	}
}

func TestGatePassesOnSmallZipLambda(t *testing.T) {
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack:      "BhRefresh-dev",
		LogicalID:  "OddsRefreshFunction26C42318",
		AssetHash:  repeat("abadcafe", 8),
		AssetBytes: 10 * 1024 * 1024,
	})
	assets, anyFail := Evaluate(cdkOut, DefaultLimits())
	if anyFail {
		t.Fatal("anyFail = true, want false")
	}
	if got := DefaultLimits().Classify(assets[0].TotalBytes()); got != "OK" {
		t.Errorf("status = %q, want OK", got)
	}
}

func TestGateWarnsBetweenThresholds(t *testing.T) {
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack:      "BhRefresh-dev",
		LogicalID:  "OddsRefreshFunction26C42318",
		AssetHash:  repeat("feedface", 8),
		AssetBytes: DefaultWarnLimitBytes + 1,
	})
	assets, anyFail := Evaluate(cdkOut, DefaultLimits())
	if anyFail {
		t.Fatal("anyFail = true, want false (WARN does not fail)")
	}
	if got := DefaultLimits().Classify(assets[0].TotalBytes()); got != "WARN" {
		t.Errorf("status = %q, want WARN", got)
	}
}

func TestImagePackageLambdaIsExempt(t *testing.T) {
	cdkOut := makeCdkOut(t, t.TempDir(), fixtureSpec{
		Stack:       "BhMcp-dev",
		LogicalID:   "McpFunctionF370A1F8",
		AssetHash:   repeat("0", 64),
		PackageType: "Image",
	})
	assets, anyFail := Evaluate(cdkOut, DefaultLimits())
	if len(assets) != 0 || anyFail {
		t.Fatalf("assets=%v anyFail=%v, want empty/false", assets, anyFail)
	}
}

func TestMalformedTemplateAndManifestDegradeGracefully(t *testing.T) {
	cdkOut := filepath.Join(t.TempDir(), "cdk.out")
	if err := os.MkdirAll(cdkOut, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(cdkOut, "BhBad-dev.template.json"),
		[]byte(`["not","an","object"]`), 0o644)
	os.WriteFile(filepath.Join(cdkOut, "BhBad-dev.assets.json"),
		[]byte(`"also-not-an-object"`), 0o644)
	assets, anyFail := Evaluate(cdkOut, DefaultLimits())
	if len(assets) != 0 || anyFail {
		t.Fatalf("assets=%v anyFail=%v, want empty/false", assets, anyFail)
	}
}

func TestSharedAssetMeasuredOncePerFunction(t *testing.T) {
	// Two functions sharing one bundle: each reported with the shared size.
	dir := t.TempDir()
	cdkOut := filepath.Join(dir, "cdk.out")
	os.MkdirAll(cdkOut, 0o755)
	hash := repeat("cafef00d", 8)
	writeSparse(t, filepath.Join(cdkOut, "asset."+hash, "shared.py"), 5000)
	writeJSON(t, filepath.Join(cdkOut, "BhShared-dev.template.json"), map[string]any{
		"Resources": map[string]any{
			"FnA": map[string]any{"Type": "AWS::Lambda::Function",
				"Properties": map[string]any{"Code": map[string]any{"S3Key": hash + ".zip"}}},
			"FnB": map[string]any{"Type": "AWS::Lambda::Function",
				"Properties": map[string]any{"Code": map[string]any{"S3Key": hash + ".zip"}}},
		},
	})
	writeJSON(t, filepath.Join(cdkOut, "BhShared-dev.assets.json"), map[string]any{
		"files": map[string]any{hash: map[string]any{
			"source": map[string]any{"path": "asset." + hash, "packaging": "zip"}}},
	})
	assets, _ := Evaluate(cdkOut, DefaultLimits())
	if len(assets) != 2 {
		t.Fatalf("len(assets) = %d, want 2", len(assets))
	}
	for _, a := range assets {
		if a.FunctionBytes != 5000 {
			t.Errorf("%s FunctionBytes = %d, want 5000", a.LogicalID, a.FunctionBytes)
		}
	}
}
