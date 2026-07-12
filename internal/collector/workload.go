package collector

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
	"github.com/Stacked-Nerds/ktrace/pkg/utils"
)

func collectStatefulSet(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	workload, err := client.Clientset.AppsV1().StatefulSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("StatefulSet", ref.Name, ref.Namespace, err)
	}
	if err := addObject(state, "StatefulSet", workload, workload.ObjectMeta); err != nil {
		return err
	}
	if err := collectPodsForOwnerWithSelector(
		ctx,
		client,
		ref.Namespace,
		string(workload.UID),
		formatLabelSelector(workload.Spec.Selector),
		state,
	); err != nil {
		state.warn(fmt.Sprintf("collect StatefulSet pods: %v", err))
	}
	return nil
}

func collectDaemonSet(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	workload, err := client.Clientset.AppsV1().DaemonSets(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("DaemonSet", ref.Name, ref.Namespace, err)
	}
	if err := addObject(state, "DaemonSet", workload, workload.ObjectMeta); err != nil {
		return err
	}
	if err := collectPodsForOwnerWithSelector(
		ctx,
		client,
		ref.Namespace,
		string(workload.UID),
		formatLabelSelector(workload.Spec.Selector),
		state,
	); err != nil {
		state.warn(fmt.Sprintf("collect DaemonSet pods: %v", err))
	}
	return nil
}

func collectJob(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	job, err := client.Clientset.BatchV1().Jobs(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("Job", ref.Name, ref.Namespace, err)
	}
	if err := addObject(state, "Job", job, job.ObjectMeta); err != nil {
		return err
	}
	if err := collectPodsForOwnerWithSelector(
		ctx,
		client,
		ref.Namespace,
		string(job.UID),
		formatLabelSelector(job.Spec.Selector),
		state,
	); err != nil {
		state.warn(fmt.Sprintf("collect Job pods: %v", err))
	}
	return nil
}

func collectCronJob(ctx context.Context, client *kubernetes.Client, ref models.ResourceRef, state *collectState) error {
	cronJob, err := client.Clientset.BatchV1().CronJobs(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return wrapNotFound("CronJob", ref.Name, ref.Namespace, err)
	}
	if err := addObject(state, "CronJob", cronJob, cronJob.ObjectMeta); err != nil {
		return err
	}

	jobs, err := client.Clientset.BatchV1().Jobs(ref.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		state.warn(fmt.Sprintf("collect CronJob history: %v", err))
		return nil
	}
	ownedJobs := make([]int, 0)
	for i := range jobs.Items {
		if utils.HasOwner(jobs.Items[i].OwnerReferences, string(cronJob.UID)) {
			ownedJobs = append(ownedJobs, i)
		}
	}
	sort.Slice(ownedJobs, func(i, j int) bool {
		left := jobs.Items[ownedJobs[i]].CreationTimestamp.Time
		right := jobs.Items[ownedJobs[j]].CreationTimestamp.Time
		return left.After(right)
	})
	const cronJobHistoryLimit = 10
	if len(ownedJobs) > cronJobHistoryLimit {
		ownedJobs = ownedJobs[:cronJobHistoryLimit]
		state.notice("CronJob history limited to 10 most recent Jobs")
	}
	for _, index := range ownedJobs {
		job := &jobs.Items[index]
		if err := addObject(state, "Job", job, job.ObjectMeta); err != nil {
			return err
		}
		if err := collectPodsForOwnerWithSelector(
			ctx,
			client,
			ref.Namespace,
			string(job.UID),
			formatLabelSelector(job.Spec.Selector),
			state,
		); err != nil {
			state.warn(fmt.Sprintf("collect pods for Job %q: %v", job.Name, err))
		}
	}
	return nil
}

// collectOwnersForPod walks the supported controller chains without expanding
// sibling workloads. Owners may already have been deleted, which is normal for
// retained pods and should not make collection fail.
func collectOwnersForPod(ctx context.Context, client *kubernetes.Client, pod models.CollectedResource, state *collectState) error {
	var owner models.OwnerRef
	found := false
	for _, candidate := range pod.Metadata.OwnerReferences {
		switch utils.NormalizeKind(candidate.Kind) {
		case "replicaset", "job":
			owner = candidate
			found = true
		}
		if found {
			break
		}
	}
	if !found {
		return nil
	}

	switch utils.NormalizeKind(owner.Kind) {
	case "replicaset":
		return collectReplicaSetOwnerChain(ctx, client, pod.Ref.Namespace, owner, state)
	case "job":
		return collectJobOwnerChain(ctx, client, pod.Ref.Namespace, owner, state)
	default:
		return nil
	}
}

func collectReplicaSetOwnerChain(ctx context.Context, client *kubernetes.Client, namespace string, owner models.OwnerRef, state *collectState) error {
	rs, err := client.Clientset.AppsV1().ReplicaSets(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("get owning ReplicaSet %q: %w", owner.Name, err)
	}
	if owner.UID != "" && string(rs.UID) != owner.UID {
		return nil
	}
	if err := addObject(state, "ReplicaSet", rs, rs.ObjectMeta); err != nil {
		return err
	}

	deploymentOwner, ok := ownerOfKind(rs.OwnerReferences, "deployment")
	if !ok {
		return nil
	}
	deployment, err := client.Clientset.AppsV1().Deployments(namespace).Get(ctx, deploymentOwner.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("get owning Deployment %q: %w", deploymentOwner.Name, err)
	}
	if deploymentOwner.UID != "" && deployment.UID != deploymentOwner.UID {
		return nil
	}
	return addObject(state, "Deployment", deployment, deployment.ObjectMeta)
}

func collectJobOwnerChain(ctx context.Context, client *kubernetes.Client, namespace string, owner models.OwnerRef, state *collectState) error {
	job, err := client.Clientset.BatchV1().Jobs(namespace).Get(ctx, owner.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("get owning Job %q: %w", owner.Name, err)
	}
	if owner.UID != "" && string(job.UID) != owner.UID {
		return nil
	}
	if err := addObject(state, "Job", job, job.ObjectMeta); err != nil {
		return err
	}

	cronJobOwner, ok := ownerOfKind(job.OwnerReferences, "cronjob")
	if !ok {
		return nil
	}
	cronJob, err := client.Clientset.BatchV1().CronJobs(namespace).Get(ctx, cronJobOwner.Name, metav1.GetOptions{})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("get owning CronJob %q: %w", cronJobOwner.Name, err)
	}
	if cronJobOwner.UID != "" && cronJob.UID != cronJobOwner.UID {
		return nil
	}
	return addObject(state, "CronJob", cronJob, cronJob.ObjectMeta)
}

func ownerOfKind(owners []metav1.OwnerReference, kind string) (metav1.OwnerReference, bool) {
	for _, owner := range owners {
		if utils.NormalizeKind(owner.Kind) == kind {
			return owner, true
		}
	}
	return metav1.OwnerReference{}, false
}

func addObject(state *collectState, kind string, object runtime.Object, metadata metav1.ObjectMeta) error {
	resource, err := toCollectedResource(kind, object, metadata)
	if err != nil {
		return err
	}
	state.add(resource)
	return nil
}

