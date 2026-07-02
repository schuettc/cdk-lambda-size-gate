package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FunctionAsset is one zip-packaged Lambda function plus the measured sizes
// of the staged assets that back it (own code + referenced layers).
type FunctionAsset struct {
	Stack            string
	LogicalID        string
	AssetHash        string
	AssetPath        string
	FunctionBytes    int64
	LayerBytes       int64 // sum of measurable referenced layer assets (Task 5)
	UnmeasuredLayers int   // layers we couldn't weigh (bare ARNs etc.) — total is a lower bound
}

// TotalBytes is what AWS enforces the 250 MiB limit against: function code
// plus all layers, combined.
func (f FunctionAsset) TotalBytes() int64 { return f.FunctionBytes + f.LayerBytes }

// asMap / asString: tolerant JSON accessors — malformed shapes yield zero
// values so a bad template degrades to "skip", never a panic.
func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }
func asString(v any) string      { s, _ := v.(string); return s }

// s3KeyHash extracts the bare asset hash from a Code.S3Key / Content.S3Key
// value. Bootstrap-synthesized templates use a plain "<hash>.zip" string;
// anything else (CFN intrinsic map, non-zip key) yields "" and is skipped.
func s3KeyHash(v any) string {
	s := asString(v)
	if !strings.HasSuffix(s, ".zip") {
		return ""
	}
	return strings.TrimSuffix(s, ".zip")
}

// assetSource resolves an asset hash to its staged source dir via the
// stack's *.assets.json manifest. Returns ("", "") when unresolvable.
func assetSource(manifest map[string]any, manifestDir, hash string) (path, packaging string) {
	entry := asMap(asMap(manifest["files"])[hash])
	source := asMap(entry["source"])
	rel := asString(source["path"])
	if rel == "" {
		return "", ""
	}
	return filepath.Join(manifestDir, rel), asString(source["packaging"])
}

// loadJSONMap reads a JSON file expecting an object; nil on any failure.
func loadJSONMap(path string) map[string]any {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return asMap(v)
}

// resolveAssetDir maps an asset hash to its staged directory, preferring the
// manifest and falling back to the conventional cdk.out/asset.<hash> dir.
// Returns "" if the asset isn't a measurable staged zip directory.
func resolveAssetDir(manifest map[string]any, cdkOut, hash string) string {
	if manifest != nil {
		if p, packaging := assetSource(manifest, cdkOut, hash); p != "" {
			if packaging != "" && packaging != "zip" {
				return ""
			}
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return p
			}
			return ""
		}
	}
	p := filepath.Join(cdkOut, "asset."+hash)
	if fi, err := os.Stat(p); err == nil && fi.IsDir() {
		return p
	}
	return ""
}

// Evaluate walks every *.template.json in cdkOut and measures every
// zip-packaged AWS::Lambda::Function (Image/container functions have a
// 10 GB limit and are exempt). Asset sizes are cached — functions sharing a
// bundle are measured once and each reported with the shared size.
func Evaluate(cdkOut string, limits Limits) (assets []FunctionAsset, anyFail bool) {
	sizeCache := map[string]int64{}
	measure := func(dir string) int64 {
		if s, ok := sizeCache[dir]; ok {
			return s
		}
		s := DirUnzippedSize(dir)
		sizeCache[dir] = s
		return s
	}

	templates, _ := filepath.Glob(filepath.Join(cdkOut, "*.template.json"))
	sort.Strings(templates)
	for _, tmpl := range templates {
		stack := strings.TrimSuffix(filepath.Base(tmpl), ".template.json")
		manifest := loadJSONMap(filepath.Join(cdkOut, stack+".assets.json"))
		doc := loadJSONMap(tmpl)
		resources := asMap(doc["Resources"])

		// Pass 1 (used from Task 5): layer logical ID → staged asset dir.
		layerDirs := map[string]string{}
		for id, r := range resources {
			res := asMap(r)
			if asString(res["Type"]) != "AWS::Lambda::LayerVersion" {
				continue
			}
			hash := s3KeyHash(asMap(asMap(res["Properties"])["Content"])["S3Key"])
			if hash == "" {
				continue
			}
			if dir := resolveAssetDir(manifest, cdkOut, hash); dir != "" {
				layerDirs[id] = dir
			}
		}

		// Pass 2: functions. Iterate in sorted-key order for stable output.
		ids := make([]string, 0, len(resources))
		for id := range resources {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			res := asMap(resources[id])
			if asString(res["Type"]) != "AWS::Lambda::Function" {
				continue
			}
			props := asMap(res["Properties"])
			if asString(props["PackageType"]) == "Image" {
				continue // container image — 10 GB limit, exempt
			}
			hash := s3KeyHash(asMap(props["Code"])["S3Key"])
			if hash == "" {
				continue // inline code or unresolvable key — nothing staged
			}
			dir := resolveAssetDir(manifest, cdkOut, hash)
			if dir == "" {
				continue
			}

			fa := FunctionAsset{
				Stack:         stack,
				LogicalID:     id,
				AssetHash:     hash,
				AssetPath:     dir,
				FunctionBytes: measure(dir),
			}
			fa.LayerBytes, fa.UnmeasuredLayers = sumLayers(props["Layers"], layerDirs, measure)
			assets = append(assets, fa)
			if limits.Classify(fa.TotalBytes()) == "FAIL" {
				anyFail = true
			}
		}
	}
	return assets, anyFail
}

// sumLayers is completed in Task 5. Until then it reports nothing measured.
func sumLayers(layers any, layerDirs map[string]string, measure func(string) int64) (int64, int) {
	return 0, 0
}
