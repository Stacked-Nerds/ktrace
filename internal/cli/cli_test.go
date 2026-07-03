package cli

import (
	"bytes"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/collector"
	"github.com/Stacked-Nerds/ktrace/internal/engine"
	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/internal/renderer/console"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestWriteReport(t *testing.T) {
	result := engine.Analyze(models.NewResourceGraph(models.ResourceRef{
		Kind: "Deployment", Name: "frontend", Namespace: "default",
	}))

	var buf bytes.Buffer
	if err := writeReport(&buf, result); err != nil {
		t.Fatalf("writeReport() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Deployment: frontend") {
		t.Errorf("report missing deployment header: %s", out)
	}
	if !strings.Contains(out, "Status:") {
		t.Errorf("report missing status: %s", out)
	}
}

func TestWriteJSON(t *testing.T) {
	result := &models.TraceResult{
		Root:   models.ResourceRef{Kind: "Pod", Name: "nginx", Namespace: "default"},
		Status: models.StatusHealthy,
	}
	var buf bytes.Buffer
	if err := writeJSON(&buf, result); err != nil {
		t.Fatalf("writeJSON() error: %v", err)
	}
	if !strings.Contains(buf.String(), `"status": "Healthy"`) {
		t.Errorf("json output missing status: %s", buf.String())
	}
}

func TestTraceWithFakeClient(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default", UID: "d1"},
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}

	origClientFn := newClientFn
	origOrchFn := newOrchestratorFn
	defer func() {
		newClientFn = origClientFn
		newOrchestratorFn = origOrchFn
	}()

	newClientFn = func(opts kubernetes.Options) (*kubernetes.Client, error) {
		return kubernetes.NewFromClientset(fake.NewSimpleClientset(deploy, ns)), nil
	}
	newOrchestratorFn = collector.NewOrchestrator

	result, err := trace("deployment", "frontend", "default")
	if err != nil {
		t.Fatalf("trace() error: %v", err)
	}
	if result.Root.Name != "frontend" {
		t.Errorf("unexpected root name: %s", result.Root.Name)
	}
}

func TestRootCommandHelp(t *testing.T) {
	cmd := newRootCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !strings.Contains(buf.String(), "deployment frontend") {
		t.Error("help text missing example")
	}
}

func TestConsoleRenderWithFinding(t *testing.T) {
	result := &models.TraceResult{
		Root:   models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Status: models.StatusFailed,
		Findings: []models.Finding{{
			Severity:  models.SeverityHigh,
			Condition: "CrashLoopBackOff",
			Summary:   "Container crash looping",
			Source:    models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
			Recommendations: []string{
				"kubectl logs app -n default",
			},
		}},
		RootCause: &models.Finding{
			Severity: models.SeverityHigh,
			Summary:  "Container crash looping",
		},
		Timeline: []models.TimelineEntry{{
			Title: "CrashLoopBackOff",
		}},
	}
	var buf bytes.Buffer
	if err := console.Render(&buf, result, console.DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Root Cause") {
		t.Fatal("expected root cause section")
	}
}
