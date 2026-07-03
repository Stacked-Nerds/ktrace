# ktrace Roadmap

## Phase 1 — Foundation (current)

- [x] Repository structure and Go module
- [x] Cobra CLI with namespace/kubeconfig/context flags
- [x] Kubernetes client wrapper
- [x] Resource collectors for Deployment, ReplicaSet, Pod, Events, PVC, PV, Node, Service, Namespace
- [x] Orchestrator graph walk
- [x] Human summary output and `--json` flag
- [x] Unit tests and benchmarks
- [x] CI (test, vet, golangci-lint)
- [x] Documentation

## Phase 2 — Timeline and Analysis

- [ ] Correlator with explicit resource edges
- [ ] Timeline builder (sorted, deduplicated, human-readable)
- [ ] Console renderer (formalized from CLI output)
- [ ] Basic analyzers:
  - ImagePullBackOff / ErrImagePull
  - CrashLoopBackOff
  - OOMKilled
  - FailedScheduling
  - PVC Pending
  - Mount failures
  - Node NotReady
- [ ] Root-cause summary and recommended `kubectl` commands

## Phase 3 — Output Formats

- [ ] JSON renderer (structured analysis output)
- [ ] Markdown renderer (`--markdown`)
- [ ] HTML renderer (`--html`)
- [ ] Additional analyzers:
  - Readiness/liveness probe failures
  - Failed volume attachment
  - Missing Secret / ConfigMap

## Future

These are not planned for immediate implementation but the architecture supports them:

- Interactive TUI
- Web UI
- Grafana / Prometheus integration
- OpenTelemetry trace correlation
- AI explanation layer
- Incident report generation (PDF export)
- Slack / GitHub Actions integration
- ArgoCD deployment timeline
- Multi-cluster support
- Collection cache and exporter packages

## Versioning

ktrace follows semantic versioning. Phase 1 ships as **v0.1.0** — API and output format may change until v1.0.
