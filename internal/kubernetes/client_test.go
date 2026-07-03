package kubernetes

import (
	"os"
	"path/filepath"
	"strings"
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
	missing := filepath.Join(t.TempDir(), "missing-kubeconfig")
	_, err := restConfig(Options{Kubeconfig: missing})
	if err == nil {
		t.Fatal("expected error for missing explicit kubeconfig")
	}
	if !isConnectError(err) {
		t.Fatalf("expected connectError, got %T: %v", err, err)
	}
}

func TestKubeconfigCandidatesExplicitPath(t *testing.T) {
	path := "/custom/kubeconfig"
	got := kubeconfigCandidates(Options{Kubeconfig: path})
	if len(got) != 1 || got[0] != path {
		t.Fatalf("kubeconfigCandidates() = %v, want [%s]", got, path)
	}
}

func TestKubeconfigCandidatesNative(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	t.Setenv("HOME", "/tmp/home")
	t.Setenv("KTRACE_RUNTIME", "")

	got := kubeconfigCandidates(Options{})
	wantPresent := map[string]bool{
		"/tmp/home/.kube/config":    false,
		"/etc/rancher/k3s/k3s.yaml": false,
	}
	wantAbsent := []string{"/root/.kube/config", "/kube/config"}

	for _, path := range got {
		if _, ok := wantPresent[path]; ok {
			wantPresent[path] = true
		}
	}
	for path, found := range wantPresent {
		if !found {
			t.Fatalf("kubeconfigCandidates() missing %q in %v", path, got)
		}
	}
	for _, path := range wantAbsent {
		for _, candidate := range got {
			if candidate == path {
				t.Fatalf("kubeconfigCandidates() should not include docker path %q", path)
			}
		}
	}
}

func TestKubeconfigCandidatesDocker(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	t.Setenv("HOME", "/tmp/home")
	t.Setenv("KTRACE_RUNTIME", "docker")

	got := kubeconfigCandidates(Options{})
	want := map[string]bool{
		"/root/.kube/config":       false,
		"/kube/config":              false,
		"/home/ktrace/.kube/config": false,
	}

	for _, path := range got {
		if _, ok := want[path]; ok {
			want[path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Fatalf("kubeconfigCandidates() missing docker path %q in %v", path, got)
		}
	}
}

func TestIsReadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !isReadableFile(path) {
		t.Fatal("expected readable file")
	}
	if isReadableFile(filepath.Join(dir, "missing")) {
		t.Fatal("expected missing file to be unreadable")
	}
}

func TestHexToIPv4(t *testing.T) {
	tests := map[string]string{
		"010011AC": "172.17.0.1",
		"00000000": "0.0.0.0",
		"bad":      "",
	}
	for hex, want := range tests {
		if got := hexToIPv4(hex); got != want {
			t.Fatalf("hexToIPv4(%q) = %q, want %q", hex, got, want)
		}
	}
}

func TestIsLoopbackServer(t *testing.T) {
	tests := map[string]bool{
		"https://127.0.0.1:6443":           true,
		"https://localhost:6443":           true,
		"https://kubernetes.default:443":     false,
		"https://192.168.1.10:6443":        false,
	}
	for server, want := range tests {
		if got := isLoopbackServer(server); got != want {
			t.Fatalf("isLoopbackServer(%q) = %v, want %v", server, got, want)
		}
	}
}

func TestReplaceServerHost(t *testing.T) {
	got, err := replaceServerHost("https://127.0.0.1:6443", "172.17.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://172.17.0.1:6443" {
		t.Fatalf("replaceServerHost() = %q", got)
	}
}

func TestServerURLCandidatesLoopback(t *testing.T) {
	t.Setenv("KTRACE_API_SERVER", "https://node.example:6443")
	t.Setenv("KTRACE_RUNTIME", "")

	got := serverURLCandidates("https://127.0.0.1:6443")
	if len(got) < 2 {
		t.Fatalf("expected multiple candidates, got %v", got)
	}
	if got[0] != "https://127.0.0.1:6443" {
		t.Fatalf("first candidate = %q, want original URL", got[0])
	}

	foundOverride := false
	for _, u := range got {
		if u == "https://node.example:6443" {
			foundOverride = true
		}
		if u == "https://host.docker.internal:6443" {
			t.Fatalf("host.docker.internal should not appear outside Docker, got %v", got)
		}
	}
	if !foundOverride {
		t.Fatalf("expected KTRACE_API_SERVER override in %v", got)
	}
}

func TestServerURLCandidatesLoopbackDocker(t *testing.T) {
	t.Setenv("KTRACE_API_SERVER", "")
	t.Setenv("KTRACE_RUNTIME", "docker")

	got := serverURLCandidates("https://127.0.0.1:6443")
	if len(got) < 2 {
		t.Fatalf("expected docker fallback candidates, got %v", got)
	}
}

func TestConnectErrorHintsBinary(t *testing.T) {
	t.Setenv("KTRACE_RUNTIME", "")

	err := newLoadError(os.ErrPermission, []string{"/home/user/.kube/config"})
	msg := err.Error()
	if !strings.Contains(msg, "permission denied") || !strings.Contains(msg, "chmod 600") {
		t.Fatalf("unexpected binary permission hint: %s", msg)
	}
}

func TestConnectErrorHintsDocker(t *testing.T) {
	t.Setenv("KTRACE_RUNTIME", "docker")

	err := newLoadError(os.ErrPermission, []string{"/root/.kube/config"})
	msg := err.Error()
	if !strings.Contains(msg, "[Docker container]") || !strings.Contains(msg, "docker run --user 0") {
		t.Fatalf("unexpected docker permission hint: %s", msg)
	}
}
