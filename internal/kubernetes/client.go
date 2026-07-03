// Package kubernetes provides a thin wrapper around the Kubernetes client-go clientset.
package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

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

// New creates a Kubernetes client from the given options.
func New(opts Options) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
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

// NewFromClientset creates a Client from an existing clientset (for testing).
func NewFromClientset(cs kubernetes.Interface) *Client {
	return &Client{Clientset: cs}
}

// DefaultNamespace returns the namespace from the current kubeconfig context.
func DefaultNamespace(opts Options) (string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if opts.Kubeconfig != "" {
		loadingRules.ExplicitPath = opts.Kubeconfig
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	if opts.Context != "" {
		configOverrides.CurrentContext = opts.Context
	}

	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	ns, _, err := clientConfig.Namespace()
	if err != nil {
		return "", fmt.Errorf("resolve default namespace: %w", err)
	}
	return ns, nil
}
