package collector

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
)

func TestOrchestratorCollectDeployment(t *testing.T) {
	deployUID := "deploy-uid-1"
	rsUID := "rs-uid-1"
	podUID := "pod-uid-1"

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend",
			Namespace: "default",
			UID:       deployUID,
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-rs",
			Namespace: "default",
			UID:       rsUID,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "frontend", UID: deployUID, Controller: boolPtr(true)},
			},
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-pod",
			Namespace: "default",
			UID:       podUID,
			Labels:    map[string]string{"app": "frontend"},
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet", Name: "frontend-rs", UID: rsUID, Controller: boolPtr(true)},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: "frontend-pvc",
						},
					},
				},
			},
		},
	}
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-pvc",
			Namespace: "default",
			UID:       "pvc-uid-1",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "frontend-pv",
		},
	}
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-pv",
			UID:  "pv-uid-1",
		},
	}
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-1",
			UID:  "node-uid-1",
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-svc",
			Namespace: "default",
			UID:       "svc-uid-1",
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "frontend"},
		},
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
			UID:  "ns-uid-1",
		},
	}
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "frontend-event",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "frontend-pod",
			Namespace: "default",
			UID:       podUID,
		},
		Type:    "Warning",
		Reason:  "FailedMount",
		Message: "Unable to attach volume",
		LastTimestamp: metav1.Time{
			Time: time.Now(),
		},
	}

	client := fake.NewSimpleClientset(deploy, rs, pod, pvc, pv, node, svc, ns, event)
	k8sClient := kubernetes.NewFromClientset(client)
	orch := NewOrchestrator(k8sClient)

	graph, err := orch.Collect(context.Background(), "deployment", "frontend", "default")
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}

	if graph.Count("Deployment") != 1 {
		t.Errorf("expected 1 deployment, got %d", graph.Count("Deployment"))
	}
	if graph.Count("ReplicaSet") != 1 {
		t.Errorf("expected 1 replicaset, got %d", graph.Count("ReplicaSet"))
	}
	if graph.Count("Pod") != 1 {
		t.Errorf("expected 1 pod, got %d", graph.Count("Pod"))
	}
	if graph.Count("PersistentVolumeClaim") != 1 {
		t.Errorf("expected 1 pvc, got %d", graph.Count("PersistentVolumeClaim"))
	}
	if graph.Count("PersistentVolume") != 1 {
		t.Errorf("expected 1 pv, got %d", graph.Count("PersistentVolume"))
	}
	if graph.Count("Node") != 1 {
		t.Errorf("expected 1 node, got %d", graph.Count("Node"))
	}
	if graph.Count("Service") != 1 {
		t.Errorf("expected 1 service, got %d", graph.Count("Service"))
	}
	if graph.Count("Namespace") != 1 {
		t.Errorf("expected 1 namespace, got %d", graph.Count("Namespace"))
	}
	if len(graph.Events) == 0 {
		t.Error("expected events to be collected")
	}
}

func TestOrchestratorUnsupportedKind(t *testing.T) {
	client := kubernetes.NewFromClientset(fake.NewSimpleClientset())
	orch := NewOrchestrator(client)

	_, err := orch.Collect(context.Background(), "ingress", "web", "default")
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}

func BenchmarkOrchestratorCollect(b *testing.B) {
	deployUID := "deploy-uid-1"
	rsUID := "rs-uid-1"

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "frontend", Namespace: "default", UID: deployUID},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend-rs", Namespace: "default", UID: rsUID,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "Deployment", Name: "frontend", UID: deployUID},
			},
		},
	}
	pods := make([]*corev1.Pod, 0, 10)
	for i := 0; i < 10; i++ {
		pods = append(pods, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "frontend-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "ReplicaSet", Name: "frontend-rs", UID: rsUID},
				},
			},
		})
	}

	objs := []runtime.Object{deploy, rs}
	for _, p := range pods {
		objs = append(objs, p)
	}
	client := fake.NewSimpleClientset(objs...)
	k8sClient := kubernetes.NewFromClientset(client)
	orch := NewOrchestrator(k8sClient)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := orch.Collect(context.Background(), "deployment", "frontend", "default")
		if err != nil {
			b.Fatalf("Collect() error: %v", err)
		}
	}
}

func boolPtr(b bool) *bool { return &b }
