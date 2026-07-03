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

	orig := inClusterNamespacePath
	inClusterNamespacePath = path
	t.Cleanup(func() { inClusterNamespacePath = defaultInClusterNamespacePath })

	if got := inClusterNamespace(); got != "production" {
		t.Fatalf("inClusterNamespace() = %q, want production", got)
	}
}
