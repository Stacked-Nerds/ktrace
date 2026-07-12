package collector

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
)

func TestOrchestratorCollectPodTraversesOwners(t *testing.T) {
	tests := []struct {
		name       string
		podOwner  metav1.OwnerReference
		objects   []runtime.Object
		wantKinds []string
	}{
		{
			name:      "replicaset to deployment",
			podOwner: metav1.OwnerReference{Kind: "ReplicaSet", Name: "web-rs", UID: "rs-uid", Controller: boolPtr(true)},
			objects: []runtime.Object{
				&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
					Name: "web-rs", Namespace: "default", UID: "rs-uid",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "Deployment", Name: "web", UID: "deployment-uid", Controller: boolPtr(true)},
					},
				}},
				&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name: "web", Namespace: "default", UID: "deployment-uid",
				}},
			},
			wantKinds: []string{"Pod", "ReplicaSet", "Deployment"},
		},
		{
			name:      "job to cronjob",
			podOwner: metav1.OwnerReference{Kind: "Job", Name: "backup-123", UID: "job-uid", Controller: boolPtr(true)},
			objects: []runtime.Object{
				&batchv1.Job{ObjectMeta: metav1.ObjectMeta{
					Name: "backup-123", Namespace: "default", UID: "job-uid",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "CronJob", Name: "backup", UID: "cronjob-uid", Controller: boolPtr(true)},
					},
				}},
				&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{
					Name: "backup", Namespace: "default", UID: "cronjob-uid",
				}},
			},
			wantKinds: []string{"Pod", "Job", "CronJob"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
				Name: "root-pod", Namespace: "default", UID: "pod-uid",
				OwnerReferences: []metav1.OwnerReference{tt.podOwner},
			}}
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", UID: "namespace-uid"}}
			objects := append([]runtime.Object{pod, namespace}, tt.objects...)
			orchestrator := NewOrchestrator(kubernetes.NewFromClientset(fake.NewSimpleClientset(objects...)))

			graph, err := orchestrator.Collect(context.Background(), "po", "root-pod", "default")
			if err != nil {
				t.Fatalf("Collect() error: %v", err)
			}
			for _, kind := range tt.wantKinds {
				if got := graph.Count(kind); got != 1 {
					t.Errorf("expected 1 %s, got %d", kind, got)
				}
			}
		})
	}
}

func TestOrchestratorCollectWorkloadRoots(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		rootName   string
		rootKind   string
		objects    []runtime.Object
		wantCounts map[string]int
	}{
		{
			name:     "statefulset alias",
			kind:     "sts",
			rootName: "db",
			rootKind: "StatefulSet",
			objects: []runtime.Object{
				&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default", UID: "sts-uid"}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "db-0", Namespace: "default", UID: "db-pod-uid",
					OwnerReferences: []metav1.OwnerReference{{Kind: "StatefulSet", Name: "db", UID: "sts-uid"}},
				}},
			},
			wantCounts: map[string]int{"StatefulSet": 1, "Pod": 1},
		},
		{
			name:     "daemonset alias",
			kind:     "ds",
			rootName: "agent",
			rootKind: "DaemonSet",
			objects: []runtime.Object{
				&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default", UID: "ds-uid"}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "agent-node-1", Namespace: "default", UID: "agent-pod-uid",
					OwnerReferences: []metav1.OwnerReference{{Kind: "DaemonSet", Name: "agent", UID: "ds-uid"}},
				}},
			},
			wantCounts: map[string]int{"DaemonSet": 1, "Pod": 1},
		},
		{
			name:       "job tolerates no child pods",
			kind:       "jobs",
			rootName:   "one-shot",
			rootKind:   "Job",
			objects:    []runtime.Object{&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "one-shot", Namespace: "default", UID: "job-root-uid"}}},
			wantCounts: map[string]int{"Job": 1, "Pod": 0},
		},
		{
			name:     "cronjob alias",
			kind:     "cj",
			rootName: "backup",
			rootKind: "CronJob",
			objects: []runtime.Object{
				&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default", UID: "cron-root-uid"}},
				&batchv1.Job{ObjectMeta: metav1.ObjectMeta{
					Name: "backup-123", Namespace: "default", UID: "cron-job-uid",
					OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "backup", UID: "cron-root-uid"}},
				}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "backup-123-pod", Namespace: "default", UID: "cron-pod-uid",
					Labels:          map[string]string{"app": "backup"},
					OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: "backup-123", UID: "cron-job-uid"}},
				}},
				&batchv1.Job{ObjectMeta: metav1.ObjectMeta{
					Name: "backup-456", Namespace: "default", UID: "cron-job-uid-2",
					OwnerReferences: []metav1.OwnerReference{{Kind: "CronJob", Name: "backup", UID: "cron-root-uid"}},
				}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Name: "backup-456-pod", Namespace: "default", UID: "cron-pod-uid-2",
					Labels:          map[string]string{"app": "backup"},
					OwnerReferences: []metav1.OwnerReference{{Kind: "Job", Name: "backup-456", UID: "cron-job-uid-2"}},
				}},
				&corev1.Service{
					ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
					Spec:       corev1.ServiceSpec{Selector: map[string]string{"app": "backup"}},
				},
			},
			wantCounts: map[string]int{"CronJob": 1, "Job": 2, "Pod": 2, "Service": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", UID: "namespace-uid"}}
			objects := append([]runtime.Object{namespace}, tt.objects...)
			orchestrator := NewOrchestrator(kubernetes.NewFromClientset(fake.NewSimpleClientset(objects...)))

			graph, err := orchestrator.Collect(context.Background(), tt.kind, tt.rootName, "default")
			if err != nil {
				t.Fatalf("Collect() error: %v", err)
			}
			if graph.Root.Kind != tt.rootKind {
				t.Errorf("root kind = %q, want %q", graph.Root.Kind, tt.rootKind)
			}
			for kind, want := range tt.wantCounts {
				if got := graph.Count(kind); got != want {
					t.Errorf("%s count = %d, want %d", kind, got, want)
				}
			}
		})
	}
}

func TestOrchestratorCollectPodToleratesDeletedOwner(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "orphan", Namespace: "default", UID: "orphan-pod-uid",
		OwnerReferences: []metav1.OwnerReference{
			{Kind: "ReplicaSet", Name: "deleted-rs", UID: "deleted-rs-uid", Controller: boolPtr(true)},
		},
	}}
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default", UID: "namespace-uid"}}
	orchestrator := NewOrchestrator(kubernetes.NewFromClientset(fake.NewSimpleClientset(pod, namespace)))

	graph, err := orchestrator.Collect(context.Background(), "pod", "orphan", "default")
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if got := graph.Count("Pod"); got != 1 {
		t.Errorf("Pod count = %d, want 1", got)
	}
	if got := graph.Count("ReplicaSet"); got != 0 {
		t.Errorf("ReplicaSet count = %d, want 0", got)
	}
}

