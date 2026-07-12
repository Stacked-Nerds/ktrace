# RBAC

ktrace is read-only. Grant only the resources you want it to diagnose.

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ktrace-reader
rules:
  - apiGroups: [""]
    resources:
      - namespaces
      - pods
      - services
      - events
      - persistentvolumeclaims
      - persistentvolumes
      - nodes
      - configmaps
      - secrets
      - serviceaccounts
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["pods/log"]
    verbs: ["get"]
  - apiGroups: ["apps"]
    resources:
      - deployments
      - replicasets
      - statefulsets
      - daemonsets
    verbs: ["get", "list"]
  - apiGroups: ["batch"]
    resources: ["jobs", "cronjobs"]
    verbs: ["get", "list"]
  - apiGroups: ["storage.k8s.io"]
    resources: ["storageclasses"]
    verbs: ["get", "list"]
```

`pods/log` is needed only when `--logs` or `--previous-logs` is used.

Secret access is used only to confirm that a referenced Secret and key exist.
ktrace discards all Secret values before they enter the resource graph. If
Secret access is denied, the trace is marked `Unknown` rather than incorrectly
reporting the Secret as missing.

For namespace-scoped installations, replace `ClusterRole` with `Role`, omit
cluster-scoped resources (Nodes, PersistentVolumes, StorageClasses), and expect
partial results for checks that require those objects.
