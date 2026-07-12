# ktrace

**Understand the story behind your Kubernetes resources.**

ktrace collects related Kubernetes resources, builds a chronological timeline, detects failure conditions, and explains the most likely root cause — so you don't have to run a dozen `kubectl` commands.

## Status

**v0.3** — Evidence-driven workload diagnostics with causal analysis, rollout
history, configuration checks, safe crash logs, and bounded collection.

## Installation

| Method | Command |
|--------|---------|
| **Go install** | `go install github.com/Stacked-Nerds/ktrace/cmd/ktrace@latest` |
| **Release binary** | [GitHub Releases](https://github.com/Stacked-Nerds/ktrace/releases) → add to `PATH` |
| **Build from source** | `make build` → `./bin/ktrace` |
| **Docker** | `docker pull ghcr.io/stacked-nerds/ktrace:latest` |
| **In-cluster Job** | See [examples/docker/README.md](examples/docker/README.md) |

Full setup for each method (k3s, kubeconfig, RBAC): **[docs/Installation.md](docs/Installation.md)**

Minimal read-only permissions and optional `pods/log` access:
**[docs/RBAC.md](docs/RBAC.md)**.

Automation schema and compatibility: **[docs/JSON.md](docs/JSON.md)**.

Connection errors include hints tailored to how you installed ktrace (binary, Docker, or in-cluster).

### Quick start — binary

```bash
kubectl cluster-info   # verify cluster access
ktrace deployment frontend -n production
```

### Quick start — Docker

```bash
docker run --rm --user 0 \
  -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

## Usage

```bash
# Trace a deployment with root-cause analysis
ktrace deployment frontend -n production

# Trace other workload controllers
ktrace statefulset database -n production
ktrace daemonset node-agent -n platform
ktrace job migration -n production
ktrace cronjob nightly-backup -n production

# Verbose output with full explanations
ktrace deployment frontend -n production -v

# Stable, redacted summary JSON
ktrace deployment frontend --json

# Include redacted raw Kubernetes snapshots when explicitly needed
ktrace deployment frontend --json --include-raw

# Add bounded logs for failing containers
ktrace deployment frontend --logs --previous-logs --log-tail 100

# Diagnosis, confidence, evidence chain, and next actions only
ktrace deployment frontend --explain

# Include collected resource counts
ktrace pod nginx -n production --show-collected
```

### Flags

| Flag | Description |
|------|-------------|
| `-n, --namespace` | Target namespace |
| `--kubeconfig` | Path to kubeconfig |
| `--context` | Kubeconfig context |
| `--json` | Export stable, redacted summary JSON |
| `--include-raw` | Include redacted raw snapshots with `--json` |
| `--explain` | Diagnosis, confidence, evidence, and next actions only |
| `--logs` | Bounded current logs for failing containers |
| `--previous-logs` | Bounded previous logs for restarted containers |
| `--log-tail` | Maximum log lines per container (default 100) |
| `--since` | Log lookback duration (default 30m) |
| `--timeout` | Overall collection deadline (default 30s) |
| `--max-resources` | Maximum retained resources (default 1000) |
| `-v, --verbose` | Detailed explanations and all recommendations |
| `--show-collected` | Show collected resource counts |

### Environment variables

| Variable | Description |
|----------|-------------|
| `KUBECONFIG` | Path to kubeconfig (same as kubectl) |
| `KTRACE_API_SERVER` | Override API server URL (useful in Docker/k3s) |
| `KTRACE_RUNTIME` | Force runtime hint mode: `docker` or `in-cluster` (testing) |

### Detected failure conditions

- ImagePullBackOff / ErrImagePull
- CrashLoopBackOff / OOMKilled / non-zero exits and signals
- Init-container and ephemeral-container failures
- Startup, readiness, and liveness probe failures
- Missing Secret, ConfigMap, key, ServiceAccount, imagePullSecret, or StorageClass
- FailedScheduling
- PVC Pending / FailedMount / ProvisioningFailed
- Node NotReady
- Deployment rollout and revision regressions
- StatefulSet, DaemonSet, Job, and CronJob failures

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | Healthy |
| `2` | Invalid usage or unsupported kind |
| `3` | Findings detected (Failed or Degraded) |
| `4` | Unknown because evidence is partial |
| `5` | Cluster connection or API error |

ktrace is read-only. Logs are opt-in and bounded. Secret values are never
collected; raw JSON, messages, and logs are redacted before output.

## Example Output

```
━━━━━━━━━━━━━━━━━━━━━━━━━━
Deployment: frontend
Namespace: production
Status: Failed
━━━━━━━━━━━━━━━━━━━━━━━━━━

Critical Issues:
  [HIGH] [PVCPending] persistentvolumeclaim/data — PVC "data" is pending
  [HIGH] [FailedMount] pod/frontend-abc — Unable to attach volume

Timeline:
  10:42  Deployment created — frontend
  10:42  ReplicaSet created — frontend-rs
  10:43  Pod created — frontend-abc
  10:44  FailedMount — Unable to attach volume
  10:45  CrashLoopBackOff — back-off restarting failed container

━━━━━━━━━━━━━━━━━━━━━━━━━━
Root Cause
PVC "data" is pending
StorageClass "longhorn" may be missing or unable to provision volume
━━━━━━━━━━━━━━━━━━━━━━━━━━
Recommendation
  kubectl get storageclass
  kubectl describe pvc data -n production
  kubectl describe pod frontend-abc -n production
```

## Development

```bash
make test
make build
# Against a disposable kind cluster:
make test-integration
```

## Architecture

See [docs/Architecture.md](docs/Architecture.md).

## Roadmap

See [docs/Roadmap.md](docs/Roadmap.md).

## License

Apache License 2.0 — see [LICENSE](LICENSE).
