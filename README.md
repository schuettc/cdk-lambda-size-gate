# cdk-lambda-size-gate

Fail CI before an oversized Lambda breaks your CDK deploy.

## The problem

CI is green. `cdk synth` is green. Code review is green. Then `cdk deploy`
dies:

```
InvalidRequest: Unzipped size must be smaller than 262144000 bytes
```

`cdk synth` stages every Lambda asset into `cdk.out` but never weighs it —
there's no size check anywhere in the synth → review → deploy pipeline. AWS
only checks at `UpdateFunctionCode` time, and it enforces a hard limit on the
**unzipped** deployment package: function code plus every attached layer,
combined, must be ≤ 262,144,000 bytes (250 MiB). A dependency bump, a new
layer, or an extra vendored asset can push you over that line with zero
warning until the deploy itself fails — often mid-pipeline, on a stack you
weren't even touching.

`cdk-lambda-size-gate` reads the already-synthesized `cdk.out` and applies
that exact AWS limit before you deploy, so the failure shows up as a CI check
instead of a broken release.

## GitHub Actions usage

The common case: run it right after `cdk synth` in your existing workflow.

```yaml
- run: npx cdk synth --quiet
- uses: schuettc/cdk-lambda-size-gate@v1
  with:
    cdk-out: cdk.out          # default
    # hard-limit: "262144000" # tunable — AWS's actual limit, default shown
    # warn-limit: "209715200" # tunable — advisory threshold, default shown
```

The action downloads a prebuilt release binary for the runner's OS/arch (no
Go toolchain required in your workflow) and runs it against `cdk-out`. It
fails the step (non-zero exit) if any Lambda is over `hard-limit`.

### Action inputs

| Input | Default | Description |
|---|---|---|
| `cdk-out` | `cdk.out` | Path to the synthesized CDK output directory |
| `hard-limit` | `262144000` | FAIL above this many unzipped bytes (AWS's actual limit) |
| `warn-limit` | `209715200` | WARN above this many unzipped bytes (never fails the build) |
| `version` | *(action ref, else latest)* | Full release tag of the binary to download, e.g. `v1.2.3` |

Pinning the action itself to a major-only tag (`@v1`) always downloads the
binary from the *latest* release — GitHub's `v1` tag has no attached release
assets, only full semver tags like `v1.0.0` do. Pinning `@v1.2.3` downloads
exactly that release's binary. The `version` input follows the same rule: it
must be a full `vX.Y.Z` tag (e.g. `v1.0.0`); anything else — a bare `1.2.3`, a
major-only tag, a branch, or a sha — falls back to the latest release instead
of the version you asked for.

## CLI usage

Install with Go:

```bash
go install github.com/schuettc/cdk-lambda-size-gate@latest
```

Or download a prebuilt binary from [GitHub Releases](https://github.com/schuettc/cdk-lambda-size-gate/releases) —
`linux`, `darwin`, and `windows`, each in `amd64` and `arm64`.

Then run it against a synthesized app:

```bash
cdk synth --quiet
cdk-lambda-size-gate -cdk-out cdk.out
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `-cdk-out` | `cdk.out` | Path to the synthesized cdk.out directory |
| `-hard-limit` | `262144000` | FAIL above this many unzipped bytes (AWS limit) |
| `-warn-limit` | `209715200` | WARN above this many unzipped bytes |

### Exit codes

| Code | Meaning |
|---|---|
| `0` | All functions OK or WARN — safe to deploy |
| `1` | At least one function exceeds `-hard-limit` — deploy would fail |
| `2` | Usage error, or `cdk.out` not found (run `cdk synth` first) |

### Example output

Run against a fixture with four functions — one clean, one near the WARN
threshold, one over the hard limit, and one carrying a bare-ARN layer that
can't be measured from `cdk.out`:

```
$ cdk-lambda-size-gate -cdk-out cdk.out
STATUS  TOTAL (MiB)    FN (MiB)   LYR (MiB)  STACK                   FUNCTION                                  ASSET
--------------------------------------------------------------------------------------------------------------------
FAIL          250.0       250.0         0.0  Demo                    FnFail                                    fn-fail
WARN          209.8       209.8         0.0  Demo                    FnWarn                                    fn-warn
OK              0.0         0.0         0.0  Demo                    FnOk                                      fn-ok
OK      ≥       0.0         0.0         0.0  Demo                    FnArnLayer                                fn-arn

≥ = function references layer(s) not staged in cdk.out (bare ARN / cross-stack);
    the true total is at least the value shown.

Thresholds: WARN > 200 MiB, FAIL > 250 MiB (AWS hard limit 262144000 bytes unzipped, function + layers).
::warning::Lambda Demo/FnWarn unzipped bundle (fn+layers) is 209.8 MiB — approaching the 250 MiB limit.
::error::Lambda Demo/FnFail unzipped bundle (fn+layers) is 250.0 MiB, over the AWS 262144000 byte (250 MiB) unzipped limit — deploy would fail.

Bundle-size gate FAILED: trim the oversized Lambda(s) above before deploy.
$ echo $?
1
```

The `::warning::` / `::error::` lines are GitHub Actions workflow-command
annotations — they surface as inline check annotations when run in the
Action, and are harmless plain text elsewhere.

## What it measures

For every `AWS::Lambda::Function` in every `*.template.json` in `cdk.out`:

- **Function code** — the unzipped size of the staged asset directory backing
  `Code.S3Key` (every file's on-disk size, summed recursively).
- **Layers** — for each entry in `Layers` that's a same-stack
  `{"Ref": <LayerVersion logicalId>}`, the layer's own staged asset is
  resolved and measured the same way, and added to the function's total. AWS
  enforces the limit against function + layers *combined*, so this tool does
  too.
- **Image-type Lambdas are exempt.** A `PackageType: Image` function has a
  separate 10 GB limit enforced by ECR/the container runtime, not the 250 MiB
  zip limit, so it's skipped entirely.
- **Unresolvable layers get a lower-bound marker.** A layer referenced by a
  bare ARN string, `Fn::ImportValue`, or any other cross-stack reference isn't
  staged in this `cdk.out` and can't be measured. Those functions are reported
  with a `≥` prefix on their total: the shown number is a floor, not the exact
  size, because the unmeasured layer(s) could still push it over the limit.
- **Symlinks count as themselves, not their target.** The size walk never
  follows a symlink — it's a directory walk (`filepath.WalkDir`), so a
  symlinked file or directory is counted at its own `lstat` size (typically a
  few bytes for the link itself). This avoids loops from self-referential
  links and avoids double-counting a target that's also walked directly.
- Functions sharing an identical staged asset (same hash) are measured once
  and reported once each, so shared dependency layers aren't double-counted
  in wall-clock cost.

## Works with any CDK language

`cdk.out` — the `*.template.json` + `*.assets.json` manifest pair this tool
reads — is produced identically by `cdk synth` regardless of which CDK
language binding wrote the app. A TypeScript, Python, Java, C#, or Go CDK app
all synth to the same manifest shape, so the gate works unmodified across all
of them. There's no app-language-specific code path in this tool at all — it
only ever looks at the synthesized output.

## Not covered

- **SAM and the Serverless Framework** — different synth output, different
  manifest format. This tool only reads CDK's `cdk.out` layout.
- **The 50 MB zipped direct-upload limit** — that's a separate, smaller limit
  that applies when you upload a zip directly under 50 MB (e.g. via the
  console or `aws lambda update-function-code --zip-file`). CDK deploys
  always stage assets through S3, so that path isn't the one being gated
  here — the 250 MiB unzipped limit is the one that actually bites CDK users.
- **Layers attached outside the stack being scanned** — a layer referenced by
  ARN (published separately, shared across stacks, or from another account)
  isn't in this `cdk.out` and can't be measured directly. Those functions
  surface with the `≥` lower-bound marker described above instead of being
  silently under-counted.
- **Non-default stack synthesizers** — this tool resolves the plain-string
  `Code.S3Key` (`"<hash>.zip"`) that CDK's modern `DefaultStackSynthesizer`
  writes. Templates from the legacy `AssetParameters`-based synthesizer, or a
  custom synthesizer that emits an intrinsic (`{"Fn::Sub": ...}`, `{"Ref":
  ...}`) `S3Key`, have unresolvable code assets and are skipped. A stack whose
  functions are all synthesized this way reports "No zip-packaged Lambda
  assets found".
