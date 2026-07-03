package runtime

import (
	"strings"
	"testing"
)

func TestDetectMethodBinary(t *testing.T) {
	inDockerFn = func() bool { return false }
	inClusterFn = func() bool { return false }
	goInstallFn = func() bool { return false }
	t.Cleanup(resetDetectors)

	if got := DetectMethod(); got != MethodBinary {
		t.Fatalf("DetectMethod() = %q, want binary", got)
	}
}

func TestDetectMethodDocker(t *testing.T) {
	inDockerFn = func() bool { return true }
	inClusterFn = func() bool { return false }
	t.Cleanup(resetDetectors)

	if got := DetectMethod(); got != MethodDocker {
		t.Fatalf("DetectMethod() = %q, want docker", got)
	}
}

func TestDetectMethodInCluster(t *testing.T) {
	inClusterFn = func() bool { return true }
	t.Cleanup(resetDetectors)

	if got := DetectMethod(); got != MethodInCluster {
		t.Fatalf("DetectMethod() = %q, want in-cluster", got)
	}
}

func TestHintsForDocker(t *testing.T) {
	h := HintsFor(MethodDocker)
	if !strings.Contains(h.NoConfig, "docker run") {
		t.Fatalf("docker noConfig hint missing docker run: %q", h.NoConfig)
	}
	if !strings.Contains(h.Loopback, "--network host") {
		t.Fatalf("docker loopback hint missing network host: %q", h.Loopback)
	}
}

func TestHintsForBinary(t *testing.T) {
	h := HintsFor(MethodBinary)
	if !strings.Contains(h.NoConfig, "kubectl cluster-info") {
		t.Fatalf("binary noConfig hint missing kubectl: %q", h.NoConfig)
	}
	if !strings.Contains(h.Loopback, "systemctl status k3s") {
		t.Fatalf("binary loopback hint missing k3s: %q", h.Loopback)
	}
}

func TestFormatHint(t *testing.T) {
	got := FormatHint(MethodDocker, "test hint")
	if !strings.Contains(got, "[Docker container]") || !strings.Contains(got, "test hint") {
		t.Fatalf("FormatHint() = %q", got)
	}
}

func resetDetectors() {
	inDockerFn = inDocker
	inClusterFn = inCluster
	goInstallFn = isGoInstall
}
