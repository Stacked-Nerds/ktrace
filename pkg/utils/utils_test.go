package utils

import (
	"testing"
)

func TestNormalizeKind(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"Deployment", "deployment"},
		{"deploy", "deployment"},
		{"rs", "replicaset"},
		{"Pod", "pod"},
		{"ns", "namespace"},
		{"service", "service"},
	}
	for _, tt := range tests {
		if got := NormalizeKind(tt.in); got != tt.want {
			t.Errorf("NormalizeKind(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestSelectorMatches(t *testing.T) {
	selector := map[string]string{"app": "frontend", "tier": "web"}
	labels := map[string]string{"app": "frontend", "tier": "web", "version": "v1"}

	if !SelectorMatches(selector, labels) {
		t.Error("expected selector to match labels")
	}

	labels["app"] = "backend"
	if SelectorMatches(selector, labels) {
		t.Error("expected selector not to match mismatched labels")
	}
}

func TestTruncate(t *testing.T) {
	if got := Truncate("hello world", 20); got != "hello world" {
		t.Errorf("unexpected truncate: %q", got)
	}
	if got := Truncate("hello world", 8); got != "hello..." {
		t.Errorf("unexpected truncate: %q", got)
	}
}
