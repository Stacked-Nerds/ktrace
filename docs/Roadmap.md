# ktrace Roadmap

## Phase 1 — Foundation (complete)

- [x] Repository structure and Go module
- [x] Cobra CLI with namespace/kubeconfig/context flags
- [x] Kubernetes client wrapper
- [x] Resource collectors for Deployment, ReplicaSet, Pod, Events, PVC, PV, Node, Service, Namespace
- [x] Orchestrator graph walk
- [x] Human summary output and `--json` flag
- [x] Unit tests and benchmarks
- [x] CI (test, vet, golangci-lint)
- [x] Documentation
- [x] Docker image publish to GHCR

## Phase 2 — Timeline and Analysis (complete)

- [x] Correlator with explicit resource edges
- [x] Timeline builder (sorted, deduplicated, human-readable)
- [x] Console renderer (status, issues, timeline, root cause, recommendations)
- [x] Basic analyzers:
  - ImagePullBackOff / ErrImagePull
  - CrashLoopBackOff
  - OOMKilled
  - FailedScheduling
  - PVC Pending
  - Mount failures
  - Node NotReady
  - Deployment condition failures
- [x] Root-cause summary and recommended `kubectl` commands
- [x] Parallel PVC/Node collection
- [x] Reduced event API calls (single list per namespace)

## Phase 3 — Output Formats

- [ ] Structured JSON analysis schema (partial — `--json` exports TraceResult)
- [ ] Markdown renderer (`--markdown`)
- [ ] HTML renderer (`--html`)
- [ ] Additional analyzers:
  - Readiness/liveness probe failures
  - Failed volume attachment
  - Missing Secret / ConfigMap

## Future

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

**v0.2.0** — Phase 2: timeline, analyzers, root-cause analysis.
