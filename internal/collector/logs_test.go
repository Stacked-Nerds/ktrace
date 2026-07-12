package collector

import (
	"encoding/json"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestFailedLogRequestsIncludesCurrentAndPrevious(t *testing.T) {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod", UID: "pod-1"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:         "api",
				RestartCount: 2,
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				},
			}},
		},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod", UID: "pod-1"},
		Raw: raw,
	})

	got := failedLogRequests(graph, LogOptions{Current: true, Previous: true})
	if len(got) != 2 {
		t.Fatalf("requests = %#v, want current and previous", got)
	}
}

func TestRedactLogBoundsLinesAndSecrets(t *testing.T) {
	input := "password=hunter2\n" + strings.Repeat("x", 5000)
	got, count := redactLog(input)
	if count != 1 {
		t.Fatalf("redactions = %d, want 1", count)
	}
	if strings.Contains(got, "hunter2") {
		t.Fatalf("secret leaked: %q", got)
	}
	if !strings.Contains(got, "[line truncated]") {
		t.Fatalf("long line was not bounded")
	}
}
