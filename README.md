# cdk-lambda-size-gate

Fail CI before an oversized Lambda breaks your CDK deploy.

`cdk synth` stages every Lambda asset but never weighs it. AWS rejects a
zip-packaged Lambda whose **unzipped** deployment package (function code +
layers) exceeds 262,144,000 bytes (250 MiB) — at deploy time, long after CI
went green. This tool reads your synthesized `cdk.out` and gates on exactly
that limit, before you deploy.

Status: pre-v1, under construction.
