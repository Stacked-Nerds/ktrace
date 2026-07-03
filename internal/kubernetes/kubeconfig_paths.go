package kubernetes

import (
	"os"
	"path/filepath"

	"github.com/Stacked-Nerds/ktrace/internal/runtime"
)

// Common kubeconfig mount paths used in Docker and legacy images.
var dockerKubeconfigPaths = []string{
	"/root/.kube/config",
	"/kube/config",
	"/home/ktrace/.kube/config",
}

// System kubeconfig paths for native installs (k3s, etc.).
var nativeKubeconfigPaths = []string{
	"/etc/rancher/k3s/k3s.yaml",
	"/etc/kubernetes/admin.conf",
}

func kubeconfigCandidates(opts Options) []string {
	if opts.Kubeconfig != "" {
		return []string{opts.Kubeconfig}
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 10)

	add := func(path string) {
		if path == "" {
			return
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}

	if env := os.Getenv("KUBECONFIG"); env != "" {
		for _, part := range filepath.SplitList(env) {
			add(part)
		}
	}

	if home := os.Getenv("HOME"); home != "" {
		add(filepath.Join(home, ".kube", "config"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(filepath.Join(home, ".kube", "config"))
	}

	if runtime.InDocker() {
		for _, path := range dockerKubeconfigPaths {
			add(path)
		}
	} else {
		for _, path := range nativeKubeconfigPaths {
			add(path)
		}
	}

	return out
}

func isReadableFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
