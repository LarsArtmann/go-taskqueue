package harvest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DefaultDiscoveryTimeout bounds one daemon discovery RPC: an unreachable or
// wedged daemon must never stall a harvest tick (the local scan is the
// fallback and costs milliseconds).
const DefaultDiscoveryTimeout = 5 * time.Second

// daemonDiscoverRequest is the POST /v1/discover request body
// (project-discovery-daemon wire protocol: sdk.DiscoverOptions, stripped to
// the field the harvester needs — no enrichment, so the daemon answers from
// its cache on the fast path).
type daemonDiscoverRequest struct {
	SearchPaths []string `json:"searchPaths"`
}

// daemonProject is one discovered project (domain.Project, stripped to the
// fields the repo mapping needs).
type daemonProject struct {
	Path    string `json:"path"`
	DirName string `json:"dirName"`
}

// daemonDiscoverResponse is the POST /v1/discover response body
// (domain.DiscoverResult, stripped to the project paths).
type daemonDiscoverResponse struct {
	Projects []daemonProject `json:"projects"`
}

// DiscoverReposDaemon enumerates candidate repos on a project-discovery-daemon
// (POST /v1/discover over the HTTP endpoint at addr, usually a unix socket
// path such as /run/project-discovery/daemon.sock) and maps the result to
// harvestable repos — absolute project paths that contain the todo file,
// deduplicated and sorted, exactly the contract DiscoverRepos upholds for the
// local scan. Addr forms: /path/to/sock and unix:///path/to/sock dial a unix
// socket, anything else is treated as a host:port TCP address (what httptest
// servers and networked daemons use).
func DiscoverReposDaemon(ctx context.Context, addr, projectsDir, todoFile string) ([]string, error) {
	client, requestURL := daemonHTTPClient(addr)

	body, err := json.Marshal(daemonDiscoverRequest{SearchPaths: []string{projectsDir}})
	if err != nil {
		return nil, fmt.Errorf("harvest: encode discovery request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		requestURL+"/v1/discover",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("harvest: create discovery request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("harvest: daemon discovery at %s: %w", addr, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		reply, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))

		return nil, fmt.Errorf(
			"harvest: daemon discovery at %s: status %d: %s",
			addr,
			resp.StatusCode,
			strings.TrimSpace(string(reply)),
		)
	}

	var decoded daemonDiscoverResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, fmt.Errorf("harvest: decode daemon discovery response: %w", err)
	}

	return mapDaemonProjects(decoded.Projects, projectsDir, todoFile), nil
}

// mapDaemonProjects maps daemon projects to harvestable repos: relative paths
// resolve against projectsDir, empty paths are dropped, duplicates collapse,
// and only repos containing the todo file survive — the same harvestability
// rule the local scan applies.
func mapDaemonProjects(projects []daemonProject, projectsDir, todoFile string) []string {
	seen := make(map[string]bool, len(projects))

	var repos []string

	for _, p := range projects {
		if p.Path == "" {
			continue
		}

		repo := p.Path
		if !filepath.IsAbs(repo) {
			repo = filepath.Join(projectsDir, repo)
		}

		if seen[repo] {
			continue
		}

		seen[repo] = true

		if info, err := os.Stat(filepath.Join(repo, todoFile)); err != nil || info.IsDir() {
			continue
		}

		repos = append(repos, repo)
	}

	sort.Strings(repos)

	return repos
}

// DiscoverReposFor resolves the candidate repos for one harvest tick: with a
// daemon addr it asks the daemon and, when that fails (socket unreachable,
// non-200, bad payload), logs ONE warning and falls back to the local scan —
// daemon mode is additive and must never fail the tick. An empty addr is the
// plain scan (the zero-external-services default).
func DiscoverReposFor(ctx context.Context, addr, projectsDir, todoFile string, log *slog.Logger) ([]string, error) {
	if addr == "" {
		return DiscoverRepos(projectsDir, todoFile)
	}

	discoverCtx, cancel := context.WithTimeout(ctx, DefaultDiscoveryTimeout)
	defer cancel()

	repos, err := DiscoverReposDaemon(discoverCtx, addr, projectsDir, todoFile)
	if err == nil {
		return repos, nil
	}

	if log != nil {
		log.Warn(
			"harvest: daemon discovery failed, falling back to local scan",
			"addr", addr,
			"err", err,
		)
	}

	return DiscoverRepos(projectsDir, todoFile)
}

// daemonHTTPClient builds the HTTP client for one daemon address: unix
// sockets dial through a custom DialContext, anything else uses the default
// transport. The request base URL is http://localhost for sockets because
// the daemon's routes carry no host semantics; explicit http(s):// and bare
// host:port addresses are used (or prefixed) as-is, which is what httptest
// servers and networked daemons need.
func daemonHTTPClient(addr string) (*http.Client, string) {
	socket, isUnix := strings.CutPrefix(addr, "unix://")

	switch {
	case isUnix:
		return unixSocketClient(socket), "http://localhost"
	case strings.HasPrefix(addr, "http://"), strings.HasPrefix(addr, "https://"):
		return http.DefaultClient, strings.TrimSuffix(addr, "/")
	case strings.ContainsRune(addr, filepath.Separator):
		return unixSocketClient(addr), "http://localhost"
	default:
		return http.DefaultClient, "http://" + addr
	}
}

// unixSocketClient dials every request over the unix socket at path.
func unixSocketClient(path string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer

				return dialer.DialContext(ctx, "unix", path)
			},
		},
	}
}
