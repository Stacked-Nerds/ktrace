# ktrace

**Understand the story behind your Kubernetes resources.**

ktrace is a Kubernetes CLI that collects related resources, correlates them into a timeline, and explains why workloads are failing — so you don't have to run a dozen `kubectl` commands to find the root cause.

## Status

**Phase 1** — Resource collection and CLI foundation. Timeline rendering and root-cause analysis arrive in Phase 2.

## Quick Start

### Docker (recommended)

Pre-built images are published to GHCR on every push to `main`:

```bash
docker pull ghcr.io/stacked-nerds/ktrace:latest

docker run --rm \
  -v "$HOME/.kube:/home/ktrace/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

See [examples/docker/README.md](examples/docker/README.md) for in-cluster Job usage and RBAC notes.

### Build from source

**Prerequisites:** Go 1.26+, `kubectl` configured, Kubernetes 1.33–1.36 (client-go v0.36)

```bash
go install github.com/Stacked-Nerds/ktrace/cmd/ktrace@latest
```

Or clone and build:

```bash
git clone https://github.com/Stacked-Nerds/ktrace.git
cd ktrace
make build
./bin/ktrace deployment frontend
```

## Usage

```bash
# Trace a deployment (default namespace from kubeconfig)
ktrace deployment frontend

# Trace a pod in a specific namespace
ktrace pod nginx -n production

# Trace a namespace (bounded collection)
ktrace namespace production

# Output full collected resource graph as JSON
ktrace deployment frontend --json

# Use a specific kubeconfig or context
ktrace deployment frontend --kubeconfig ~/.kube/config --context prod
```

### Supported root resource types (Phase 1)

| Type | Description |
|------|-------------|
| `deployment` | Full ownership chain: Deployment → ReplicaSet → Pod → Events → PVC → PV → Node → Service |
| `replicaset` | ReplicaSet → Pod → related resources |
| `pod` | Pod → PVC, PV, Node, Events, Services |
| `namespace` | Namespace + bounded Deployments/Pods |

## Example Output

```
━━━━━━━━━━━━━━━━━━━━━━━━━━
Deployment: frontend
Namespace: production
━━━━━━━━━━━━━━━━━━━━━━━━━━

Collected:
  ReplicaSets:  2
  Pods:         3
  Events:       12
  PVCs:         1
  PVs:          1
  Nodes:        2
  Services:     1
  Deployments:  1

Recent Events:
  10:45  Warning  FailedMount  pod/frontend-abc  Unable to attach volume...

(Timeline and root cause analysis coming in Phase 2)
```

## Development

```bash
make test    # run tests with race detector
make vet     # go vet
make lint    # golangci-lint
make build   # build binary to bin/ktrace
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for contribution guidelines.

## Architecture

See [docs/Architecture.md](docs/Architecture.md) for package layout and design.

## Roadmap

See [docs/Roadmap.md](docs/Roadmap.md) for planned features.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
