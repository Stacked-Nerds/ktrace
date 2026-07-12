# JSON output

`ktrace --json` emits the redacted summary schema
`ktrace.dev/v1alpha1`. It is intended for CI, scripts, and incident tooling.

Top-level fields:

- `schemaVersion`
- `root`
- `status`
- `partial` and `warnings`
- `diagnosis` (root cause, confidence, evidence chain, contributors, symptoms)
- `findings`
- `timeline`
- optional bounded `logs`
- `resourceCounts`
- `collectedAt`

Raw Kubernetes objects are omitted. Use `--json --include-raw` only when a
full resource snapshot is required. Raw mode is still redacted:

- Secret and ConfigMap data
- service-account tokens
- literal environment values
- common password, token, API-key, authorization, AWS-key, and JWT patterns

Consumers must check `schemaVersion` before parsing. Additive fields may be
introduced within `v1alpha1`; incompatible changes will use a new schema
version.

## Status and exit code

- `Healthy` → exit `0`
- `Failed` or `Degraded` → exit `3`
- `Unknown` / partial evidence → exit `4`

Connection and API failures that prevent root collection exit `5`; invalid
usage exits `2`.
