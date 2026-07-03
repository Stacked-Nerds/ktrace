package errors

import "testing"

func TestNotFound(t *testing.T) {
	err := NotFound("Deployment", "frontend", "default")
	if !IsNotFound(err) {
		t.Error("expected not found error")
	}
}

func TestUnsupportedKind(t *testing.T) {
	err := UnsupportedKind("ingress")
	if !IsUnsupportedKind(err) {
		t.Error("expected unsupported kind error")
	}
}

func TestInvalidArgs(t *testing.T) {
	err := InvalidArgs("missing name")
	if !IsInvalidArgs(err) {
		t.Error("expected invalid args error")
	}
}
