# Installation

ktrace supports several installation methods. Connection errors include hints
tailored to how you are running the tool.

After installation, verify Kubernetes access and the binary:

```bash
kubectl cluster-info
ktrace --version
ktrace deployment <name> -n <namespace> --explain
```

Supported workload roots in v0.3 are Deployment, ReplicaSet, Pod, Namespace,
StatefulSet, DaemonSet, Job, and CronJob. See [RBAC.md](RBAC.md) when running
with a restricted user or ServiceAccount.

## 1. Go install (recommended for developers)

**Prerequisites:** Go 1.26+

```bash
go install github.com/Stacked-Nerds/ktrace/cmd/ktrace@latest
```

Ensure `$(go env GOPATH)/bin` is on your `PATH`. ktrace uses the same kubeconfig
as `kubectl` (`~/.kube/config` or `KUBECONFIG`).

```bash
kubectl cluster-info   # verify access first
ktrace deployment frontend -n production
```

## 2. Download release binary

Download the binary for your OS/arch from
[GitHub Releases](https://github.com/Stacked-Nerds/ktrace/releases), then:

```bash
chmod +x ktrace
sudo mv ktrace /usr/local/bin/   # or any directory on your PATH
```

Configure cluster access the same way as kubectl:

```bash
export KUBECONFIG=~/.kube/config   # optional if default path exists
ktrace deployment frontend -n production
```

### k3s

```bash
mkdir -p ~/.kube
sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config
sudo chown "$USER" ~/.kube/config
chmod 600 ~/.kube/config
ktrace deployment frontend -n default
```

## 3. Build from source

```bash
git clone https://github.com/Stacked-Nerds/ktrace.git
cd ktrace
make build
./bin/ktrace deployment frontend -n production
```

Or install locally:

```bash
make install
ktrace deployment frontend -n production
```

## 4. Docker

```bash
docker pull ghcr.io/stacked-nerds/ktrace:latest

docker run --rm --user 0 \
  -v "$HOME/.kube:/root/.kube:ro" \
  ghcr.io/stacked-nerds/ktrace:latest \
  deployment frontend -n production
```

ktrace auto-discovers kubeconfig inside the container and retries localhost API
URLs via the Docker host gateway. See [examples/docker/README.md](../examples/docker/README.md).

## 5. In-cluster Job

Run ktrace inside the cluster when you want to trace from CI or without local
kubeconfig:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: ktrace-frontend
spec:
  template:
    spec:
      serviceAccountName: ktrace
      restartPolicy: Never
      containers:
        - name: ktrace
          image: ghcr.io/stacked-nerds/ktrace:latest
          args: ["deployment", "frontend", "-n", "production"]
```

Grant the `ktrace` ServiceAccount read-only RBAC on traced resources.

## Troubleshooting

| Symptom | Binary / go install | Docker |
|---------|----------------------|--------|
| No kubeconfig | `kubectl cluster-info`, set `KUBECONFIG` | Mount `-v $HOME/.kube:/root/.kube:ro` |
| Permission denied | `chmod 600 ~/.kube/config` | `docker run --user 0 ...` |
| Connection refused (127.0.0.1) | Check cluster: `systemctl status k3s` | `--network host` or `KTRACE_API_SERVER` |

Override the API server without editing kubeconfig:

```bash
export KTRACE_API_SERVER=https://<node-ip>:6443
```

Force runtime detection for testing:

```bash
export KTRACE_RUNTIME=docker   # or: in-cluster
```
