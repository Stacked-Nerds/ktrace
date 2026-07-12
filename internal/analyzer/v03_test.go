package analyzer

import (
	"encoding/json"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestPodDiagnosticsTable(t *testing.T) {
	tests := []struct {
		name      string
		pod       corev1.Pod
		events    []models.TimelineEvent
		condition string
		container string
	}{
		{
			name: "init container create error",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{{
					Name: "setup",
					State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
						Reason: "CreateContainerError", Message: "failed to create container",
					}},
				}}},
			},
			condition: "CreateContainerError", container: "setup",
		},
		{
			name: "init container non-zero exit",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Status: corev1.PodStatus{InitContainerStatuses: []corev1.ContainerStatus{{
					Name: "migrate",
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						Reason: "Error", ExitCode: 42,
					}},
				}}},
			},
			condition: "InitContainerFailed", container: "migrate",
		},
		{
			name: "ephemeral container signal",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Status: corev1.PodStatus{EphemeralContainerStatuses: []corev1.ContainerStatus{{
					Name: "debugger",
					State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						ExitCode: 143, Signal: 15,
					}},
				}}},
			},
			condition: "ContainerSignaled", container: "debugger",
		},
		{
			name: "restart history",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
					Name: "app", RestartCount: 3,
					LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
						Reason: "Error", ExitCode: 2,
					}},
				}}},
			},
			condition: "ContainerRestartHistory", container: "app",
		},
		{
			name: "pod eviction",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Status:     corev1.PodStatus{Reason: "Evicted", Message: "node low on ephemeral-storage"},
			},
			condition: "PodEvicted",
		},
		{
			name: "startup probe event",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			},
			events: []models.TimelineEvent{{
				Source: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
				Reason: "Unhealthy", Message: "Startup probe failed: connection refused",
			}},
			condition: "StartupProbeFailed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
			addObject(t, graph, "Pod", "app", "default", &test.pod)
			graph.Events = test.events
			finding, ok := findingByCondition(Analyze(graph), test.condition)
			if !ok {
				t.Fatalf("expected condition %q", test.condition)
			}
			if test.container != "" && finding.Container != test.container {
				t.Fatalf("expected container %q, got %q", test.container, finding.Container)
			}
		})
	}
}

func TestReferenceDiagnosticsTable(t *testing.T) {
	trueValue := true
	tests := []struct {
		name      string
		build     func(*testing.T, *models.ResourceGraph)
		condition string
		want      bool
	}{
		{
			name: "missing ConfigMap",
			build: func(t *testing.T, graph *models.ResourceGraph) {
				pod := corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name: "app", EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}},
						}},
					}}},
				}
				addObject(t, graph, "Pod", "app", "default", &pod)
				graph.References = append(graph.References, models.ResourceReference{
					From: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
					To: models.ResourceRef{Kind: "ConfigMap", Name: "settings", Namespace: "default"},
					FieldPath: "spec.containers[0].envFrom[0]", Observed: true,
				})
			},
			condition: "MissingConfigMap", want: true,
		},
		{
			name: "optional ConfigMap",
			build: func(t *testing.T, graph *models.ResourceGraph) {
				pod := corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
					Spec: corev1.PodSpec{Containers: []corev1.Container{{
						Name: "app", EnvFrom: []corev1.EnvFromSource{{
							ConfigMapRef: &corev1.ConfigMapEnvSource{
								LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}, Optional: &trueValue,
							},
						}},
					}}},
				}
				addObject(t, graph, "Pod", "app", "default", &pod)
				graph.References = append(graph.References, models.ResourceReference{
					From: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
					To: models.ResourceRef{Kind: "ConfigMap", Name: "settings", Namespace: "default"},
					FieldPath: "spec.containers[0].envFrom[0]", Optional: true, Observed: true,
				})
			},
			condition: "MissingConfigMap", want: false,
		},
		{
			name: "existing Secret payload is not decoded",
			build: func(t *testing.T, graph *models.ResourceGraph) {
				pod := corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
					Spec: corev1.PodSpec{
						ImagePullSecrets: []corev1.LocalObjectReference{{Name: "registry"}},
						Containers:       []corev1.Container{{Name: "app"}},
					},
				}
				addObject(t, graph, "Pod", "app", "default", &pod)
				graph.AddResource(models.CollectedResource{
					Ref: models.ResourceRef{Kind: "Secret", Name: "registry", Namespace: "default"},
					Raw: json.RawMessage(`{"data": this-is-intentionally-not-json}`),
				})
				graph.References = append(graph.References, models.ResourceReference{
					From: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
					To: models.ResourceRef{Kind: "Secret", Name: "registry", Namespace: "default"},
					FieldPath: "spec.imagePullSecrets[0]", Observed: true, TargetExists: true, Exists: true,
				})
			},
			condition: "MissingSecret", want: false,
		},
		{
			name: "missing ServiceAccount",
			build: func(t *testing.T, graph *models.ResourceGraph) {
				deployment := appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
					Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{ServiceAccountName: "runner", Containers: []corev1.Container{{Name: "app"}}},
					}},
				}
				addObject(t, graph, "Deployment", "app", "default", &deployment)
				graph.References = append(graph.References, models.ResourceReference{
					From: models.ResourceRef{Kind: "Deployment", Name: "app", Namespace: "default"},
					To: models.ResourceRef{Kind: "ServiceAccount", Name: "runner", Namespace: "default"},
					FieldPath: "spec.template.spec.serviceAccountName", Observed: true,
				})
			},
			condition: "MissingServiceAccount", want: true,
		},
		{
			name: "missing StorageClass",
			build: func(t *testing.T, graph *models.ResourceGraph) {
				class := "fast"
				pvc := corev1.PersistentVolumeClaim{
					ObjectMeta: metav1.ObjectMeta{Name: "data", Namespace: "default"},
					Spec:       corev1.PersistentVolumeClaimSpec{StorageClassName: &class},
					Status:     corev1.PersistentVolumeClaimStatus{Phase: corev1.ClaimBound},
				}
				addObject(t, graph, "PersistentVolumeClaim", "data", "default", &pvc)
				graph.References = append(graph.References, models.ResourceReference{
					From: models.ResourceRef{Kind: "PersistentVolumeClaim", Name: "data", Namespace: "default"},
					To: models.ResourceRef{Kind: "StorageClass", Name: "fast"},
					FieldPath: "spec.storageClassName", Observed: true,
				})
			},
			condition: "MissingStorageClass", want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
			test.build(t, graph)
			_, got := findingByCondition(Analyze(graph), test.condition)
			if got != test.want {
				t.Fatalf("condition %q presence = %t, want %t", test.condition, got, test.want)
			}
		})
	}
}

func TestReferenceDiagnosticsDistinguishesMissingKey(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	graph.References = []models.ResourceReference{{
		From:         models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		To:           models.ResourceRef{Kind: "Secret", Name: "database", Namespace: "default"},
		FieldPath:    "spec.containers[0].env[0]",
		Key:          "password",
		Observed:     true,
		TargetExists: true,
		Exists:       false,
	}}

	finding, ok := findingByCondition(Analyze(graph), "MissingSecretKey")
	if !ok {
		t.Fatal("expected MissingSecretKey")
	}
	if finding.Source.Kind != "Secret" || finding.Source.Name != "database" {
		t.Fatalf("source = %#v", finding.Source)
	}
}

func TestRolloutAndRecentChangesTable(t *testing.T) {
	replicas := int32(2)
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", Generation: 4},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 3,
			Conditions: []appsv1.DeploymentCondition{{
				Type: appsv1.DeploymentProgressing, Status: corev1.ConditionFalse,
				Reason: "ProgressDeadlineExceeded", Message: "rollout exceeded its progress deadline",
			}},
		},
	}
	previousSpec := corev1.PodSpec{
		ServiceAccountName: "old",
		NodeSelector:       map[string]string{"pool": "stable"},
		Containers: []corev1.Container{{
			Name: "app", Image: "example/app:v1",
		}},
	}
	currentSpec := corev1.PodSpec{
		ServiceAccountName: "new",
		NodeSelector:       map[string]string{"pool": "canary"},
		Containers: []corev1.Container{{
			Name: "app", Image: "example/app:v2",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
			},
			ReadinessProbe: &corev1.Probe{InitialDelaySeconds: 5},
			EnvFrom: []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "settings"}},
			}},
		}},
	}
	owner := []metav1.OwnerReference{{Kind: "Deployment", Name: "app"}}
	previous := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app-old", Namespace: "default", Annotations: map[string]string{revisionAnnotation: "1"}, OwnerReferences: owner},
		Spec:       appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: previousSpec}},
	}
	current := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Name: "app-new", Namespace: "default", Annotations: map[string]string{revisionAnnotation: "2"}, OwnerReferences: owner},
		Spec:       appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: currentSpec}},
	}

	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "app", Namespace: "default"})
	addObject(t, graph, "Deployment", "app", "default", &deployment)
	addObject(t, graph, "ReplicaSet", "app-old", "default", &previous)
	addObject(t, graph, "ReplicaSet", "app-new", "default", &current)
	findings := Analyze(graph)

	expected := []string{
		"ObservedGenerationLag",
		"StalledRollout",
		"RecentChangeImages",
		"RecentChangeResources",
		"RecentChangeProbes",
		"RecentChangeConfigReferences",
		"RecentChangeServiceAccount",
		"RecentChangeSchedulingConstraints",
	}
	for _, condition := range expected {
		t.Run(condition, func(t *testing.T) {
			finding, ok := findingByCondition(findings, condition)
			if !ok {
				t.Fatalf("expected condition %q", condition)
			}
			if len(condition) >= len("RecentChange") && condition[:len("RecentChange")] == "RecentChange" &&
				finding.Category != "Correlation" {
				t.Fatalf("expected correlation category, got %q", finding.Category)
			}
		})
	}
}

func TestRecentChangesDoNotDegradeHealthyRollout(t *testing.T) {
	owner := []metav1.OwnerReference{{Kind: "Deployment", Name: "app"}}
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Status:     appsv1.DeploymentStatus{AvailableReplicas: 1, ReadyReplicas: 1},
	}
	previous := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-old", Namespace: "default",
			Annotations: map[string]string{revisionAnnotation: "1"}, OwnerReferences: owner,
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v1"}}},
		}},
	}
	current := appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "app-new", Namespace: "default",
			Annotations: map[string]string{revisionAnnotation: "2"}, OwnerReferences: owner,
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "app:v2"}}},
		}},
	}
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Deployment", Name: "app", Namespace: "default"})
	addObject(t, graph, "Deployment", "app", "default", &deployment)
	addObject(t, graph, "ReplicaSet", "app-old", "default", &previous)
	addObject(t, graph, "ReplicaSet", "app-new", "default", &current)

	if _, ok := findingByCondition(Analyze(graph), "RecentChangeImages"); ok {
		t.Fatal("healthy rollout should not emit a failure finding for image history")
	}
}

func TestMemoryLimitDecreaseIsCalledOut(t *testing.T) {
	previous := corev1.PodSpec{Containers: []corev1.Container{{
		Name: "app",
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		}},
	}}}
	current := corev1.PodSpec{Containers: []corev1.Container{{
		Name: "app",
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("64Mi"),
		}},
	}}}
	if !memoryLimitDecreased(current, previous) {
		t.Fatal("expected decreased memory limit to be detected")
	}
}

func TestWorkloadConditionDiagnosticsTable(t *testing.T) {
	suspended := true
	tests := []struct {
		name      string
		kind      string
		object    interface{}
		condition string
	}{
		{
			name: "StatefulSet condition", kind: "StatefulSet", condition: "ReplicaFailure",
			object: &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
				Status: appsv1.StatefulSetStatus{Conditions: []appsv1.StatefulSetCondition{{
					Type: appsv1.StatefulSetConditionType("ReplicaFailure"), Status: corev1.ConditionTrue, Reason: "CreateFailed",
				}}},
			},
		},
		{
			name: "DaemonSet condition", kind: "DaemonSet", condition: "ReplicaFailure",
			object: &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
				Status: appsv1.DaemonSetStatus{Conditions: []appsv1.DaemonSetCondition{{
					Type: appsv1.DaemonSetConditionType("ReplicaFailure"), Status: corev1.ConditionTrue, Reason: "CreateFailed",
				}}},
			},
		},
		{
			name: "Job failed", kind: "Job", condition: "JobFailed",
			object: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{Name: "migration", Namespace: "default"},
				Status: batchv1.JobStatus{Conditions: []batchv1.JobCondition{{
					Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded",
				}}},
			},
		},
		{
			name: "CronJob suspended", kind: "CronJob", condition: "CronJobSuspended",
			object: &batchv1.CronJob{
				ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
				Spec:       batchv1.CronJobSpec{Suspend: &suspended},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			graph := models.NewResourceGraph(models.ResourceRef{Kind: test.kind, Name: "root", Namespace: "default"})
			addObject(t, graph, test.kind, "root", "default", test.object)
			if _, ok := findingByCondition(Analyze(graph), test.condition); !ok {
				t.Fatalf("expected condition %q", test.condition)
			}
		})
	}
}

func TestAnalyzeDeterministicAndDeduplicated(t *testing.T) {
	graph := models.NewResourceGraph(models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"})
	event := models.TimelineEvent{
		Source: models.ResourceRef{Kind: "Pod", Name: "app", Namespace: "default"},
		Reason: "Unhealthy", Message: "Liveness probe failed: timeout",
	}
	graph.Events = []models.TimelineEvent{event, event}

	first := Analyze(graph)
	second := Analyze(graph)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Analyze returned findings in a non-deterministic order")
	}
	count := 0
	for _, finding := range first {
		if finding.Condition == "LivenessProbeFailed" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one deduplicated probe finding, got %d", count)
	}
}

func addObject(t *testing.T, graph *models.ResourceGraph, kind, name, namespace string, object interface{}) {
	t.Helper()
	raw, err := json.Marshal(object)
	if err != nil {
		t.Fatalf("marshal %s: %v", kind, err)
	}
	graph.AddResource(models.CollectedResource{
		Ref: models.ResourceRef{Kind: kind, Name: name, Namespace: namespace},
		Raw: raw,
	})
}

func findingByCondition(findings []models.Finding, condition string) (models.Finding, bool) {
	for _, finding := range findings {
		if finding.Condition == condition {
			return finding, true
		}
	}
	return models.Finding{}, false
}
