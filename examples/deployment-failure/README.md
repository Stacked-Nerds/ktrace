# Deployment Failure Example

This example demonstrates a deployment that fails due to a missing StorageClass (`longhorn`).

## Setup

```bash
kubectl apply -f manifests.yaml
```

## Trace with ktrace

```bash
ktrace deployment frontend -n production
```

Expected Phase 1 output includes:

- Collected PVC referencing `longhorn` StorageClass
- Events such as `FailedMount` or PVC pending/provisioning failures

## Cleanup

```bash
kubectl delete -f manifests.yaml
```

## Phase 2 Preview

In Phase 2, ktrace will correlate this into a timeline and report:

```
Root Cause: StorageClass "longhorn" unavailable.
Suggested Fix:
  kubectl get storageclass
  kubectl describe pvc frontend-data -n production
```
