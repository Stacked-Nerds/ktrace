package collector

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"

	"github.com/Stacked-Nerds/ktrace/internal/kubernetes"
	"github.com/Stacked-Nerds/ktrace/internal/redact"
	"github.com/Stacked-Nerds/ktrace/pkg/models"
)

const defaultLogBytes int64 = 64 * 1024
const defaultTotalLogBytes int64 = 1024 * 1024

// LogOptions controls opt-in, bounded container log collection.
type LogOptions struct {
	Current       bool
	Previous      bool
	TailLines     int64
	SinceSeconds  int64
	MaxBytes      int64
	MaxTotalBytes int64
	Concurrency   int
}

// CollectFailureLogs fetches logs only for failed or restarting containers.
func CollectFailureLogs(
	ctx context.Context,
	client *kubernetes.Client,
	graph *models.ResourceGraph,
	opts LogOptions,
) {
	if graph == nil || (!opts.Current && !opts.Previous) {
		return
	}
	if opts.TailLines <= 0 {
		opts.TailLines = 100
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = defaultLogBytes
	}
	if opts.MaxTotalBytes <= 0 {
		opts.MaxTotalBytes = defaultTotalLogBytes
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 4
	}

	requests := failedLogRequests(graph, opts)
	if len(requests) == 0 {
		return
	}
	maxRequests := int(opts.MaxTotalBytes / opts.MaxBytes)
	if maxRequests < 1 {
		maxRequests = 1
	}
	if len(requests) > maxRequests {
		requests = requests[:maxRequests]
		graph.Partial = true
		graph.Warnings = appendUnique(
			graph.Warnings,
			fmt.Sprintf("log collection budget reached; limited to %d container log streams", maxRequests),
		)
	}

	sem := make(chan struct{}, opts.Concurrency)
	results := make(chan logResult, len(requests))
	var wg sync.WaitGroup
	for _, request := range requests {
		request := request
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- logResult{err: ctx.Err(), request: request}
				return
			}
			results <- fetchLog(ctx, client, request, opts)
		}()
	}
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			graph.Partial = true
			graph.Warnings = appendUnique(
				graph.Warnings,
				fmt.Sprintf("collect logs for %s/%s: %v", result.request.pod.Name, result.request.container, result.err),
			)
			continue
		}
		graph.Logs = append(graph.Logs, result.log)
	}
	sort.Slice(graph.Logs, func(i, j int) bool {
		left := graph.Logs[i].Pod.String() + "|" + graph.Logs[i].Container + fmt.Sprintf("|%t", graph.Logs[i].Previous)
		right := graph.Logs[j].Pod.String() + "|" + graph.Logs[j].Container + fmt.Sprintf("|%t", graph.Logs[j].Previous)
		return left < right
	})
}

type logRequest struct {
	pod       models.ResourceRef
	container string
	previous  bool
}

type logResult struct {
	request logRequest
	log     models.ContainerLog
	err     error
}

func failedLogRequests(graph *models.ResourceGraph, opts LogOptions) []logRequest {
	requests := make([]logRequest, 0)
	seen := make(map[string]bool)
	for _, resource := range graph.Resources["Pod"] {
		var pod corev1.Pod
		if err := decodeRaw(resource.Raw, &pod); err != nil {
			continue
		}
		ref := models.ResourceRef{
			Kind: "Pod", Name: pod.Name, Namespace: pod.Namespace, UID: string(pod.UID),
		}
		statuses := append([]corev1.ContainerStatus{}, pod.Status.InitContainerStatuses...)
		statuses = append(statuses, pod.Status.ContainerStatuses...)
		statuses = append(statuses, pod.Status.EphemeralContainerStatuses...)
		for _, status := range statuses {
			if !isFailedContainer(status) {
				continue
			}
			if opts.Current {
				addLogRequest(&requests, seen, logRequest{pod: ref, container: status.Name})
			}
			if opts.Previous && status.RestartCount > 0 {
				addLogRequest(&requests, seen, logRequest{pod: ref, container: status.Name, previous: true})
			}
		}
	}
	return requests
}

func addLogRequest(requests *[]logRequest, seen map[string]bool, request logRequest) {
	key := request.pod.String() + "/" + request.container + fmt.Sprintf("/%t", request.previous)
	if seen[key] {
		return
	}
	seen[key] = true
	*requests = append(*requests, request)
}

func isFailedContainer(status corev1.ContainerStatus) bool {
	return status.RestartCount > 0 ||
		status.State.Waiting != nil ||
		(status.State.Terminated != nil && status.State.Terminated.ExitCode != 0) ||
		(status.LastTerminationState.Terminated != nil && status.LastTerminationState.Terminated.ExitCode != 0)
}

func fetchLog(
	ctx context.Context,
	client *kubernetes.Client,
	request logRequest,
	opts LogOptions,
) logResult {
	podOpts := &corev1.PodLogOptions{
		Container: request.container,
		Previous:  request.previous,
		TailLines: &opts.TailLines,
	}
	if opts.SinceSeconds > 0 {
		podOpts.SinceSeconds = &opts.SinceSeconds
	}

	stream, err := client.Clientset.CoreV1().Pods(request.pod.Namespace).GetLogs(request.pod.Name, podOpts).Stream(ctx)
	if err != nil {
		return logResult{request: request, err: err}
	}
	defer stream.Close()

	data, err := io.ReadAll(io.LimitReader(stream, opts.MaxBytes+1))
	if err != nil {
		return logResult{request: request, err: err}
	}
	truncated := int64(len(data)) > opts.MaxBytes
	if truncated {
		data = data[:opts.MaxBytes]
	}
	content, redactions := redactLog(string(data))
	return logResult{
		request: request,
		log: models.ContainerLog{
			Pod:        request.pod,
			Container:  request.container,
			Previous:   request.previous,
			Content:    content,
			Truncated:  truncated,
			Redactions: redactions,
		},
	}
}

func redactLog(content string) (string, int) {
	content, redactions := redact.Text(content)

	// Bound pathological single lines even inside the byte budget.
	var out strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 1024), 256*1024)
	first := true
	for scanner.Scan() {
		if !first {
			out.WriteByte('\n')
		}
		first = false
		line := scanner.Text()
		if len(line) > 4096 {
			line = line[:4096] + " …[line truncated]"
		}
		out.WriteString(line)
	}
	return out.String(), redactions
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
