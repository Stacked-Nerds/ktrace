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
	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestWriteSummary(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{
		Kind: "Deployment", Name: "frontend", Namespace: "default",
	})
	graph.AddResource(models.CollectedResource{Ref: models.ResourceRef{Kind: "Pod", Name: "p1"}})

	var buf bytes.Buffer
	if err := writeSummary(&buf, graph); err != nil {
		t.Fatalf("writeSummary() error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Deployment: frontend") {
		t.Errorf("summary missing deployment header: %s", out)
	}
	if !strings.Contains(out, "Phase 2") {
		t.Errorf("summary missing phase 2 note: %s", out)
	}
}

func TestWriteJSON(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{
		Kind: "Pod", Name: "nginx", Namespace: "default",
	})
	var buf bytes.Buffer
	if err := writeJSON(&buf, graph); err != nil {
		t.Fatalf("writeJSON() error: %v", err)
	}
	if !strings.Contains(buf.String(), `"kind": "Pod"`) {
		t.Errorf("json output missing root kind: %s", buf.String())
	}
}

func TestCollectWithFakeClient(t *testing.T) {
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

	graph, err := collect("deployment", "frontend", "default")
	if err != nil {
		t.Fatalf("collect() error: %v", err)
	}
	if graph.Root.Name != "frontend" {
		t.Errorf("unexpected root name: %s", graph.Root.Name)
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
