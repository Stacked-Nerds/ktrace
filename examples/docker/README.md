# Running ktrace from Docker

Images are published to GitHub Container Registry:

```
ghcr.io/stacked-nerds/ktrace:latest
```

## Pull the image

```bash
docker pull ghcr.io/stacked-nerds/ktrace:latest
```

If the package is private, authenticate first:

```bash
echo $GITHUB_TOKEN | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

## Run with a kubeconfig (from your machine)

Mount your kubeconfig so ktrace can reach the cluster API:

```bash
docker run --rm \
  -v "$HOME/.kube:/home/ktrace/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

Or point at a specific config file:

```bash
docker run --rm \
  -v /path/to/kubeconfig:/config:ro \
  -e KUBECONFIG=/config \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production --json
```

## Run inside Kubernetes (in-cluster)

When the pod runs inside the cluster, client-go uses the service account token automatically:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: ktrace-frontend
  namespace: default
spec:
  template:
    spec:
      serviceAccountName: ktrace
      restartPolicy: Never
      containers:
        - name: ktrace
          image: ghcr.io/stacked-nerds/ktrace:latest
          args:
            - deployment
            - frontend
            - -n
            - production
```

The `ktrace` service account needs read-only RBAC for the resources you trace (`get`, `list` on deployments, pods, events, etc.).
