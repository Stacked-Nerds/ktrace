package kubernetes

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Stacked-Nerds/ktrace/internal/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const connectTimeout = 5 * time.Second

func needsConnectionProbe(cfg *rest.Config) bool {
	if runtime.InCluster() {
		return false
	}
	if os.Getenv("KTRACE_API_SERVER") != "" {
		return true
	}
	if isLoopbackServer(cfg.Host) {
		return true
	}
	if runtime.InDocker() {
		return true
	}
	return false
}

func verifyClusterConnection(cfg *rest.Config) (*rest.Config, error) {
	if !needsConnectionProbe(cfg) {
		return cfg, nil
	}

	servers := serverURLCandidates(cfg.Host)
	var tried []string
	var lastErr error

	for _, server := range servers {
		tried = append(tried, server)

		trial := rest.CopyConfig(cfg)
		trial.Host = server
		trial.Timeout = connectTimeout

		if err := pingAPI(trial); err != nil {
			lastErr = err
			continue
		}
		return trial, nil
	}

	return nil, newConnectError(lastErr, tried)
}

func pingAPI(cfg *rest.Config) error {
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	defer cancel()

	_, err = client.Discovery().RESTClient().Get().AbsPath("/version").Do(ctx).Raw()
	return err
}

func serverURLCandidates(serverURL string) []string {
	out := []string{serverURL}
	if !isLoopbackServer(serverURL) {
		return dedupeStrings(out)
	}

	if override := strings.TrimSpace(os.Getenv("KTRACE_API_SERVER")); override != "" {
		out = append(out, override)
	}

	for _, host := range loopbackHostAlternatives() {
		if alt, err := replaceServerHost(serverURL, host); err == nil {
			out = append(out, alt)
		}
	}

	return dedupeStrings(out)
}

func loopbackHostAlternatives() []string {
	if !runtime.InDocker() {
		return nil
	}

	out := make([]string, 0, 3)

	if gateway := hostGatewayIP(); gateway != "" {
		out = append(out, gateway)
	}
	out = append(out, "host.docker.internal")

	return out
}

func isLoopbackServer(serverURL string) bool {
	u, err := url.Parse(serverURL)
	if err != nil {
		return strings.Contains(serverURL, "127.0.0.1") || strings.Contains(serverURL, "localhost")
	}

	host := u.Hostname()
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func replaceServerHost(serverURL, newHost string) (string, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	port := u.Port()
	if port == "" {
		u.Host = newHost
	} else {
		u.Host = net.JoinHostPort(newHost, port)
	}

	return u.String(), nil
}

// hostGatewayIP returns the default route gateway (Docker bridge host) on Linux.
func hostGatewayIP() string {
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(data), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		return hexToIPv4(fields[2])
	}

	return ""
}

func hexToIPv4(hexIP string) string {
	if len(hexIP) != 8 {
		return ""
	}

	ip := make(net.IP, 4)
	for i := range ip {
		var octet byte
		if _, err := fmt.Sscanf(hexIP[6-i*2:8-i*2], "%02x", &octet); err != nil {
			return ""
		}
		ip[i] = octet
	}

	return ip.String()
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
