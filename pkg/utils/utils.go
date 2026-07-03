// Package utils provides small helper functions for ktrace.
package utils

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NormalizeKind returns a canonical lowercase kind name.
func NormalizeKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	switch k {
	case "deploy", "deployments":
		return "deployment"
	case "rs", "replicasets":
		return "replicaset"
	case "pods":
		return "pod"
	case "ns", "namespaces":
		return "namespace"
	default:
		return k
	}
}

// HasOwner checks whether obj is owned by the given owner UID.
func HasOwner(owners []metav1.OwnerReference, ownerUID string) bool {
	for _, o := range owners {
		if string(o.UID) == ownerUID {
			return true
		}
	}
	return false
}

// OwnerUIDs returns all owner UIDs from owner references.
func OwnerUIDs(owners []metav1.OwnerReference) []string {
	uids := make([]string, 0, len(owners))
	for _, o := range owners {
		uids = append(uids, string(o.UID))
	}
	return uids
}

// SelectorMatches returns true if all selector labels exist on obj labels with matching values.
func SelectorMatches(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// Truncate truncates s to max runes, appending "..." if truncated.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
