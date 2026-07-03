package kubernetes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInClusterNamespaceFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "namespace")
	if err := os.WriteFile(path, []byte("production\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	inClusterNamespacePath = path
	t.Cleanup(func() { inClusterNamespacePath = defaultInClusterNamespacePath })

	if got := inClusterNamespace(); got != "production" {
		t.Fatalf("inClusterNamespace() = %q, want production", got)
	}
}

func TestRestConfigExplicitKubeconfigDoesNotFallback(t *testing.T) {
	_, err := restConfig(Options{
		Kubeconfig: filepath.Join(t.TempDir(), "missing-kubeconfig"),
	})
	if err == nil {
		t.Fatal("expected error for missing explicit kubeconfig")
	}
}
