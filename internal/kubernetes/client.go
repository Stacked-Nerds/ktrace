// Package kubernetes provides a thin wrapper around the Kubernetes client-go clientset.
package kubernetes

import (
	"fmt"
	"os"
	"strings"

	"github.com/Stacked-Nerds/ktrace/internal/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultInClusterNamespacePath = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

var inClusterNamespacePath = defaultInClusterNamespacePath

// Client wraps a Kubernetes clientset with REST configuration.
type Client struct {
	Clientset kubernetes.Interface
	Config    *rest.Config
}

// Options holds client configuration options.
type Options struct {
	Kubeconfig string
	Context    string
	Namespace  string
}

// New creates a Kubernetes client from kubeconfig or in-cluster credentials.
func New(opts Options) (*Client, error) {
	config, err := restConfig(opts)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	return &Client{
		Clientset: clientset,
		Config:    config,
	}, nil
}

func restConfig(opts Options) (*rest.Config, error) {
	explicitPath := opts.Kubeconfig != ""

	if explicitPath {
		cfg, err := loadClientConfig(opts.Kubeconfig, opts.Context)
		if err != nil {
			return nil, newLoadError(fmt.Errorf("load kubeconfig %q: %w", opts.Kubeconfig, err), []string{opts.Kubeconfig})
		}
		return verifyClusterConnection(cfg)
	}

	if cfg, err := tryDefaultKubeconfigLoad(opts.Context); err == nil {
		if connected, err := verifyClusterConnection(cfg); err == nil {
			return connected, nil
		} else if isConnectError(err) {
			return nil, err
		}
	}

	candidates := kubeconfigCandidates(opts)
	var triedPaths []string
	var lastLoadErr error

	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
		}

		triedPaths = append(triedPaths, path)

		cfg, err := loadClientConfig(path, opts.Context)
		if err != nil {
			lastLoadErr = err
			if isPermissionError(err) {
				continue
			}
			continue
		}

		connected, err := verifyClusterConnection(cfg)
		if err == nil {
			return connected, nil
		}
		if isConnectError(err) {
			return nil, err
		}
		lastLoadErr = err
	}

	if len(triedPaths) == 0 {
		return restConfigInCluster(lastLoadErr, candidates)
	}

	if lastLoadErr != nil {
		return nil, newLoadError(lastLoadErr, triedPaths)
	}

	return restConfigInCluster(nil, triedPaths)
}

func tryDefaultKubeconfigLoad(context string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return clientConfig.ClientConfig()
}

func restConfigInCluster(kubeErr error, triedPaths []string) (*rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		combined := fmt.Errorf("kubeconfig: %v; in-cluster: %w", kubeErr, err)
		if runtime.InCluster() {
			return nil, newInClusterLoadError(combined, triedPaths)
		}
		return nil, newLoadError(combined, triedPaths)
	}
	return config, nil
}

func loadClientConfig(path, context string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	return clientConfig.ClientConfig()
}

// DefaultNamespace returns the namespace from kubeconfig or the in-cluster service account.
func DefaultNamespace(opts Options) (string, error) {
	explicitPath := opts.Kubeconfig != ""

	if explicitPath {
		return namespaceFromConfig(opts.Kubeconfig, opts.Context)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		overrides.CurrentContext = opts.Context
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	if ns, _, err := clientConfig.Namespace(); err == nil && ns != "" {
		return ns, nil
	}

	for _, path := range kubeconfigCandidates(opts) {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if ns, err := namespaceFromConfig(path, opts.Context); err == nil && ns != "" {
			return ns, nil
		}
	}

	if ns := inClusterNamespace(); ns != "" {
		return ns, nil
	}

	return "", fmt.Errorf("resolve default namespace: no kubeconfig or in-cluster namespace found")
}

func namespaceFromConfig(path, context string) (string, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: path}
	overrides := &clientcmd.ConfigOverrides{}
	if context != "" {
		overrides.CurrentContext = context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	ns, _, err := clientConfig.Namespace()
	return ns, err
}

// NewFromClientset creates a Client from an existing clientset (for testing).
func NewFromClientset(cs kubernetes.Interface) *Client {
	return &Client{Clientset: cs}
}

func inClusterNamespace() string {
	data, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
