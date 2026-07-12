package redact

import (
	"strings"
	"testing"
)

func TestTextRedactsCredentials(t *testing.T) {
	input := "password=hunter2 Authorization: Bearer abc.def.ghi token=secret-token"
	got, count := Text(input)
	if count < 2 {
		t.Fatalf("redactions = %d, want at least 2", count)
	}
	if strings.Contains(got, "hunter2") || strings.Contains(got, "secret-token") {
		t.Fatalf("credential leaked: %q", got)
	}
}

func TestJSONRedactsSecretAndEnvironmentValues(t *testing.T) {
	input := []byte(`{
		"kind":"Secret",
		"data":{"password":"encoded"},
		"spec":{"containers":[{"env":[{"name":"PASSWORD","value":"cleartext"}]}]},
		"messages":["token=another-secret"]
	}`)
	got := string(JSON(input))
	if strings.Contains(got, "encoded") || strings.Contains(got, "cleartext") ||
		strings.Contains(got, "another-secret") {
		t.Fatalf("sensitive JSON leaked: %s", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("missing redaction marker: %s", got)
	}
}
