package runtime

import "fmt"

// ConnectHints returns installation-specific troubleshooting hints.
type ConnectHints struct {
	NoConfig   string
	Permission string
	Loopback   string
	InCluster  string
}

// HintsFor returns hints tailored to the detected install method.
func HintsFor(method Method) ConnectHints {
	switch method {
	case MethodDocker:
		return ConnectHints{
			NoConfig: "No readable kubeconfig found inside the container.\n" +
				"  docker run --rm --user 0 -v $HOME/.kube:/root/.kube:ro ghcr.io/stacked-nerds/ktrace:latest deployment <name> -n <ns>\n" +
				"  Or set KUBECONFIG=/root/.kube/config if your mount path differs from HOME.",
			Permission: "Kubeconfig exists but is not readable by the container user (often mode 600 on the host).\n" +
				"  docker run --rm --user 0 -v $HOME/.kube:/root/.kube:ro ...\n" +
				"  Or run as your host user: docker run --user \"$(id -u):$(id -g)\" -v $HOME/.kube:/kube:ro -e KUBECONFIG=/kube/config ...",
			Loopback: "Kubeconfig points at localhost, which refers to the container — not the host running k3s/kubectl.\n" +
				"  docker run --network host --user 0 -v $HOME/.kube:/root/.kube:ro ...\n" +
				"  Or set KTRACE_API_SERVER=https://<node-ip>:6443 without editing kubeconfig.",
		}
	case MethodInCluster:
		return ConnectHints{
			NoConfig: "No kubeconfig found and in-cluster credentials are unavailable.\n" +
				"  Ensure the pod sets serviceAccountName and automountServiceAccountToken: true.\n" +
				"  The service account token should be mounted at /var/run/secrets/kubernetes.io/serviceaccount/token.",
			Permission: "Cannot read Kubernetes credentials inside the pod.\n" +
				"  Check pod securityContext and projected service account token volume permissions.",
			InCluster: "Failed to load in-cluster configuration.\n" +
				"  Run ktrace as a Kubernetes Job with a dedicated ServiceAccount and RBAC (get/list on traced resources).\n" +
				"  See examples/docker/README.md for an in-cluster Job manifest.",
		}
	case MethodGoInstall:
		return ConnectHints{
			NoConfig: "No kubeconfig found for your Go-installed ktrace binary.\n" +
				"  Ensure kubectl works first: kubectl cluster-info\n" +
				"  Default path: ~/.kube/config — or export KUBECONFIG=/path/to/config\n" +
				"  k3s: mkdir -p ~/.kube && sudo cp /etc/rancher/k3s/k3s.yaml ~/.kube/config && sudo chown $USER ~/.kube/config",
			Permission: "Cannot read kubeconfig (permission denied).\n" +
				"  chmod 600 ~/.kube/config && chown $(whoami) ~/.kube/config\n" +
				"  Or pass an explicit path: ktrace --kubeconfig ~/.kube/config deployment <name> -n <ns>",
			Loopback: "Cannot reach the API server at localhost.\n" +
				"  Verify the cluster is running: kubectl cluster-info\n" +
				"  k3s: sudo systemctl status k3s\n" +
				"  minikube: minikube status\n" +
				"  If kubectl works but ktrace does not, pass --kubeconfig explicitly.",
		}
	default:
		return ConnectHints{
			NoConfig: "No kubeconfig found.\n" +
				"  Ensure kubectl works: kubectl cluster-info\n" +
				"  Default: ~/.kube/config — or export KUBECONFIG=/path/to/config\n" +
				"  k3s: cp /etc/rancher/k3s/k3s.yaml ~/.kube/config (see docs/Installation.md)",
			Permission: "Cannot read kubeconfig (permission denied).\n" +
				"  chmod 600 ~/.kube/config\n" +
				"  Or use: ktrace --kubeconfig /path/to/config deployment <name> -n <ns>",
			Loopback: "Cannot reach the API server at localhost.\n" +
				"  Is the cluster running? kubectl cluster-info\n" +
				"  k3s: sudo systemctl status k3s | minikube: minikube status\n" +
				"  Confirm the server URL in kubeconfig matches a reachable address.",
		}
	}
}

// FormatHint prefixes a hint with the install method label.
func FormatHint(method Method, hint string) string {
	if hint == "" {
		return ""
	}
	return fmt.Sprintf("[%s] %s", method.Label(), hint)
}
