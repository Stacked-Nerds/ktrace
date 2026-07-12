package collector

import (
	"context"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

func TestCollectConfigurationReferencesRecordsMissingSecret(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod", UID: "pod-1"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "api",
				Env: []corev1.EnvVar{{
					Name: "PASSWORD",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "database"},
							Key:                  "password",
						},
					},
				}},
			}},
		},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	state := newCollectState(models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod"})
	state.add(models.CollectedResource{
		Ref:      models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod", UID: "pod-1"},
		Raw:      raw,
		Metadata: models.ResourceMeta{UID: "pod-1"},
	})

	client := kubernetes.NewFromClientset(fake.NewSimpleClientset(pod))
	collectConfigurationReferences(context.Background(), client, state)

	if len(state.graph.References) != 1 {
		t.Fatalf("references = %#v", state.graph.References)
	}
	ref := state.graph.References[0]
	if ref.To.Kind != "Secret" || ref.To.Name != "database" || !ref.Observed || ref.Exists {
		t.Fatalf("reference = %#v", ref)
	}
}

func TestCollectConfigurationReferencesRetainsSecretKeysNotValues(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "prod", UID: "secret-1"},
		Data:       map[string][]byte{"password": []byte("super-secret")},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "prod", UID: "pod-1"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name: "api",
				EnvFrom: []corev1.EnvFromSource{{
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{Name: "database"},
					},
				}},
			}},
		},
	}
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	state := newCollectState(models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod"})
	state.add(models.CollectedResource{
		Ref:      models.ResourceRef{Kind: "Pod", Name: "api", Namespace: "prod", UID: "pod-1"},
		Raw:      raw,
		Metadata: models.ResourceMeta{UID: "pod-1"},
	})

	client := kubernetes.NewFromClientset(fake.NewSimpleClientset(pod, secret))
	collectConfigurationReferences(context.Background(), client, state)

	resources := state.graph.Resources["Secret"]
	if len(resources) != 1 {
		t.Fatalf("secret resources = %#v", resources)
	}
	if string(resources[0].Raw) == "" || containsBytes(resources[0].Raw, []byte("super-secret")) {
		t.Fatalf("secret value leaked in raw: %s", resources[0].Raw)
	}
	if !containsBytes(resources[0].Raw, []byte(`"password"`)) {
		t.Fatalf("secret key not retained: %s", resources[0].Raw)
	}
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
