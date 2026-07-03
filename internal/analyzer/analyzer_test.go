package analyzer

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestAnalyzePVCPending(t *testing.T) {
	sc := "longhorn"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "default"},
		Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &sc},
		Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimPending},
	}
	raw, _ := json.Marshal(pvc)

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "app", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "PersistentVolumeClaim", Name: "data", Namespace: "default"},
		Raw: raw,
	})

	findings := Analyze(graph)
	if len(findings) == 0 {
		t.Fatal("expected PVC pending finding")
	}
	if findings[0].Condition != "PVCPending" {
		t.Fatalf("unexpected condition: %s", findings[0].Condition)
	}
}

func TestAnalyzeCrashLoopBackOff(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: "back-off restarting"},
				},
			}},
		},
	}
	raw, _ := json.Marshal(pod)

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Raw: raw,
	})

	findings := Analyze(graph)
	found := false
	for _, f := range findings {
		if f.Condition == "CrashLoopBackOff" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected CrashLoopBackOff finding")
	}
}

func TestAnalyzeOOMKilledFromLastState(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: "app",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137},
				},
			}},
		},
	}
	raw, _ := json.Marshal(pod)

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Raw: raw,
	})

	findings := Analyze(graph)
	found := false
	for _, f := range findings {
		if f.Condition == "OOMKilled" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected OOMKilled finding from LastState")
	}
}

func TestAnalyzeSchedulingDedupesConditionAndEvent(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{{
				Type:    corev1.PodScheduled,
				Status:  corev1.ConditionFalse,
				Reason:  "Unschedulable",
				Message: "0/1 nodes available",
			}},
		},
	}
	raw, _ := json.Marshal(pod)

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Raw: raw,
	})
	graph.Events = []models.TimelineEvent{{
		Reason:  "FailedScheduling",
		Message: "0/1 nodes available",
		Source:  models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
	}}

	findings := analyzeScheduling(graph)
	count := 0
	for _, f := range findings {
		if f.Condition == "FailedScheduling" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 FailedScheduling finding, got %d", count)
	}
}
