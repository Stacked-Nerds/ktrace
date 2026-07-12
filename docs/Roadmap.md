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

## v0.3 — Workload Diagnostics

- [x] Graph-aware diagnosis with confidence and evidence chains
- [x] Upward Pod ownership traversal
- [x] StatefulSet, DaemonSet, Job, and CronJob roots
- [x] Init-container, probe, exit-code, eviction, and restart analysis
- [x] Missing Secret, ConfigMap, key, ServiceAccount, imagePullSecret, and StorageClass analysis
- [x] Rollout revision and configuration-change correlation
- [x] Opt-in bounded current/previous logs with redaction
- [x] Collection timeout, cancellation, budgets, and partial/Unknown status
- [x] Stable summary JSON and explicit redacted raw output
- [x] Automation-friendly exit codes and `--explain`
- [x] kind integration scenarios

## v0.4 — Traffic Path Diagnostics

- [ ] Service → EndpointSlice → Pod path analysis
- [ ] Ingress and Gateway API (`Accepted`, `ResolvedRefs`, `Programmed`)
- [ ] NetworkPolicy and DNS diagnostics
- [ ] TLS Secret and backend-port validation

## v0.5 — Scaling and Disruption

- [ ] HPA metrics/target diagnostics
- [ ] PodDisruptionBudget rollout and drain blockers
- [ ] ResourceQuota and LimitRange failures
- [ ] Topology spread, affinity, taint, and capacity explanations

## Output Formats

- [ ] Markdown renderer (`--markdown`)
- [ ] HTML renderer (`--html`)

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

**v0.2.0** — Timeline, analyzers, root-cause analysis.

**v0.3.0** — Evidence-driven workload diagnostics and safe automation.
