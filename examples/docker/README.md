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

## Run with a kubeconfig (recommended)

Mount your kubeconfig and run. ktrace automatically tries common paths
(`/root/.kube/config`, `/kube/config`) and, for k3s-style localhost URLs,
retries via the Docker host gateway — so this often works without extra flags:

```bash
docker run --rm \
  -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

If kubeconfig is mode `600` (typical), run as root so the file is readable:

```bash
docker run --rm --user 0 \
  -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

For k3s when auto-retry still cannot reach the API server, use host networking:

```bash
docker run --rm --network host --user 0 \
  -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

Override the API server URL without editing kubeconfig:

```bash
docker run --rm --user 0 \
  -v "$HOME/.kube:/root/.kube:ro" \
  -e KTRACE_API_SERVER=https://<NODE_IP>:6443 \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

### Shell alias

```bash
alias ktrace='docker run --rm --user 0 -v $HOME/.kube:/root/.kube:ro ghcr.io/stacked-nerds/ktrace:latest'

ktrace deployment ai-backend-api -n api
ktrace deployment ai-backend-api -n api -v --json
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
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
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
