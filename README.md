# ktrace

**Understand the story behind your Kubernetes resources.**

ktrace collects related Kubernetes resources, builds a chronological timeline, detects failure conditions, and explains the most likely root cause — so you don't have to run a dozen `kubectl` commands.

## Status

**Phase 2** — Timeline, failure analysis, root-cause detection, and actionable recommendations.

## Quick Start

### Docker (recommended)

```bash
docker pull ghcr.io/stacked-nerds/ktrace:latest

docker run --rm \
  -v "$HOME/.kube:/home/ktrace/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

See [examples/docker/README.md](examples/docker/README.md) for in-cluster usage.

### Build from source

**Prerequisites:** Go 1.26+, `kubectl` configured, Kubernetes 1.33–1.36

```bash
go install github.com/Stacked-Nerds/ktrace/cmd/ktrace@latest
```

## Usage

```bash
# Trace a deployment with root-cause analysis
ktrace deployment frontend -n production

# Verbose output with full explanations
ktrace deployment frontend -n production -v

# Full structured JSON (graph, timeline, findings, root cause)
ktrace deployment frontend --json

# Include collected resource counts
ktrace pod nginx -n production --show-collected
```

### Flags

| Flag | Description |
|------|-------------|
| `-n, --namespace` | Target namespace |
| `--kubeconfig` | Path to kubeconfig |
| `--context` | Kubeconfig context |
| `--json` | Export full `TraceResult` as JSON |
| `-v, --verbose` | Detailed explanations and all recommendations |
| `--show-collected` | Show collected resource counts |

### Detected failure conditions

- ImagePullBackOff / ErrImagePull
- CrashLoopBackOff / OOMKilled
- FailedScheduling
- PVC Pending / FailedMount / ProvisioningFailed
- Node NotReady
- Deployment unavailable / not progressing

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
```

## Architecture

See [docs/Architecture.md](docs/Architecture.md).

## Roadmap

See [docs/Roadmap.md](docs/Roadmap.md).

## License

Apache License 2.0 — see [LICENSE](LICENSE).
