// Package kubernetes provides a thin wrapper around the Kubernetes client-go clientset.
package kubernetes

import (
	"fmt"
	"os"
	"strings"

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
	explicit := opts.Kubeconfig != "" || opts.Context != ""

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	} else if env := os.Getenv("KUBECONFIG"); env != "" {
		loadingRules.ExplicitPath = env
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	config, kubeErr := clientConfig.ClientConfig()
	if kubeErr == nil {
		return config, nil
	}
	if explicit {
		return nil, fmt.Errorf("load kubeconfig: %w", kubeErr)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubernetes config (kubeconfig: %v; in-cluster: %w)", kubeErr, err)
	}
	return config, nil
}

// NewFromClientset creates a Client from an existing clientset (for testing).
func NewFromClientset(cs kubernetes.Interface) *Client {
	return &Client{Clientset: cs}
}

// DefaultNamespace returns the namespace from kubeconfig or the in-cluster service account.
func DefaultNamespace(opts Options) (string, error) {
	explicit := opts.Kubeconfig != "" || opts.Context != ""

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	if ns, _, err := clientConfig.Namespace(); err == nil && ns != "" {
		return ns, nil
	} else if explicit {
		return "", fmt.Errorf("resolve default namespace from kubeconfig: %w", err)
	}

	if ns := inClusterNamespace(); ns != "" {
		return ns, nil
	}

	return "", fmt.Errorf("resolve default namespace: no kubeconfig or in-cluster namespace found")
}

func inClusterNamespace() string {
	data, err := os.ReadFile(inClusterNamespacePath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
