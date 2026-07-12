# Workload diagnostic integration tests

These scenarios intentionally create broken workloads in namespace
`ktrace-e2e`:

- Deployment with a missing Secret
- Deployment with a failing init container
- Deployment with a failing readiness probe
- Deployment that exceeds its memory limit
- Job that exceeds its backoff limit
- StatefulSet with a missing StorageClass

Run against a disposable cluster:

```bash
make build
make test-integration
```

The test script expects ktrace exit code `3` (findings) or `4` (partial
evidence), and verifies the expected diagnosis appears in summary JSON.

Do not apply these manifests to a production cluster.
