package runtime

import (
	"os"
	"path/filepath"
	"strings"
)

// Method describes how ktrace is running.
type Method string

const (
	MethodBinary    Method = "binary"
	MethodGoInstall Method = "go-install"
	MethodDocker    Method = "docker"
	MethodInCluster Method = "in-cluster"
)

var (
	inDockerFn  = inDocker
	inClusterFn = inCluster
	goInstallFn = isGoInstall
)

// DetectMethod returns the detected install/runtime method.
func DetectMethod() Method {
	if inClusterFn() {
		return MethodInCluster
	}
	if inDockerFn() {
		return MethodDocker
	}
	if goInstallFn() {
		return MethodGoInstall
	}
	return MethodBinary
}

// Label returns a short human-readable label for error messages.
func (m Method) Label() string {
	switch m {
	case MethodGoInstall:
		return "Go-installed binary"
	case MethodDocker:
		return "Docker container"
	case MethodInCluster:
		return "in-cluster pod"
	default:
		return "installed binary"
	}
}

func inDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return strings.EqualFold(os.Getenv("KTRACE_RUNTIME"), "docker")
}

func inCluster() bool {
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return true
	}
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		return true
	}
	return strings.EqualFold(os.Getenv("KTRACE_RUNTIME"), "in-cluster")
}

func isGoInstall() bool {
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	exe = filepath.ToSlash(filepath.Clean(exe))

	if gobin := os.Getenv("GOBIN"); gobin != "" {
		if strings.HasPrefix(exe, filepath.ToSlash(filepath.Clean(gobin))) {
			return true
		}
	}

	if gopath := os.Getenv("GOPATH"); gopath != "" {
		if strings.HasPrefix(exe, filepath.ToSlash(filepath.Join(gopath, "bin"))) {
			return true
		}
	}

	if home, err := os.UserHomeDir(); err == nil {
		defaultGoBin := filepath.ToSlash(filepath.Join(home, "go", "bin"))
		if strings.HasPrefix(exe, defaultGoBin) {
			return true
		}
	}

	return false
}

// InDocker reports whether ktrace is running inside a Docker container.
func InDocker() bool {
	return inDockerFn()
}

// InCluster reports whether ktrace is running inside a Kubernetes pod.
func InCluster() bool {
	return inClusterFn()
}
