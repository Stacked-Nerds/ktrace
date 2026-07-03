package kubernetes

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Stacked-Nerds/ktrace/internal/runtime"
)

type connectError struct {
	method       runtime.Method
	cause        error
	serverURLs   []string
	kubeconfig   []string
	permission   bool
	loopback     bool
	noConfig     bool
	inCluster    bool
}

func (e *connectError) Error() string {
	var b strings.Builder
	b.WriteString("connect to cluster: ")
	if e.cause != nil {
		b.WriteString(e.cause.Error())
	} else {
		b.WriteString("unable to reach Kubernetes API")
	}

	if len(e.kubeconfig) > 0 {
		b.WriteString(fmt.Sprintf(" (kubeconfig tried: %s)", strings.Join(e.kubeconfig, ", ")))
	}
	if len(e.serverURLs) > 0 {
		b.WriteString(fmt.Sprintf(" (API server tried: %s)", strings.Join(e.serverURLs, ", ")))
	}

	if hint := e.hint(); hint != "" {
		b.WriteString("\n\n")
		b.WriteString(hint)
	}

	return b.String()
}

func (e *connectError) Unwrap() error {
	return e.cause
}

func (e *connectError) hint() string {
	hints := runtime.HintsFor(e.method)

	switch {
	case e.inCluster:
		return runtime.FormatHint(e.method, hints.InCluster)
	case e.noConfig:
		return runtime.FormatHint(e.method, hints.NoConfig)
	case e.permission:
		return runtime.FormatHint(e.method, hints.Permission)
	case e.loopback:
		return runtime.FormatHint(e.method, hints.Loopback)
	default:
		return ""
	}
}

func newConnectError(cause error, serverURLs []string) *connectError {
	method := runtime.DetectMethod()
	return &connectError{
		method:     method,
		cause:      cause,
		serverURLs: serverURLs,
		loopback:   containsLoopback(serverURLs) && method != runtime.MethodInCluster,
	}
}

func newLoadError(cause error, kubeconfigPaths []string) error {
	method := runtime.DetectMethod()
	err := &connectError{
		method:     method,
		cause:      cause,
		kubeconfig: kubeconfigPaths,
		permission: isPermissionError(cause),
		noConfig:   isNoConfigError(cause) || len(kubeconfigPaths) == 0,
	}
	return err
}

func newInClusterLoadError(cause error, kubeconfigPaths []string) error {
	return &connectError{
		method:     runtime.MethodInCluster,
		cause:      cause,
		kubeconfig: kubeconfigPaths,
		inCluster:  true,
	}
}

func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	if os.IsPermission(err) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "permission denied")
}

func isNoConfigError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no configuration has been provided") ||
		strings.Contains(msg, "invalid configuration")
}

func containsLoopback(urls []string) bool {
	for _, u := range urls {
		if isLoopbackServer(u) {
			return true
		}
	}
	return false
}

func isConnectError(err error) bool {
	var ce *connectError
	return errors.As(err, &ce)
}
