// The installed pull gate serves one source-verified runtime image from an
// owned, read-only Distribution store. It cannot upload, redirect or proxy to
// an arbitrary host. The caller owns the helper container and its lifecycle.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	repository        = "luci/runtime-installed"
	frontAddress      = "0.0.0.0:5000"
	backendAddress    = "127.0.0.1:5001"
	storeDirectory    = "/registry-data"
	allowlistPath     = "/registry-data/runtime-pull-allowlist.json"
	registryBinary    = "/usr/local/bin/registry"
	configPath        = "/run/registry/config.yml"
	maxAllowlistBytes = 1 << 20
)

var digestRE = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var hexRE = regexp.MustCompile(`^[a-f0-9]{64}$`)
var rangeRE = regexp.MustCompile(`^bytes=[0-9]+-$`)

type blob struct {
	Digest string `json:"digest"`
	Bytes  int64  `json:"bytes"`
}

type allowlist struct {
	Version        int    `json:"version"`
	Repository     string `json:"repository"`
	ManifestDigest string `json:"manifestDigest"`
	ManifestBytes  int64  `json:"manifestBytes"`
	ConfigDigest   string `json:"configDigest"`
	ConfigBytes    int64  `json:"configBytes"`
	Layers         []blob `json:"layers"`
}

func decodeAllowlist(data []byte, expectedHash string) (allowlist, error) {
	var a allowlist
	if !hexRE.MatchString(expectedHash) || len(data) == 0 || len(data) > maxAllowlistBytes {
		return a, errors.New("pull allowlist digest or size invalid")
	}
	h := sha256.Sum256(data)
	if hex.EncodeToString(h[:]) != expectedHash {
		return a, errors.New("pull allowlist bytes changed")
	}
	if err := uniqueJSON(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return a, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&a); err != nil {
		return a, err
	}
	if decoder.Decode(new(any)) != io.EOF || a.Version != 1 || a.Repository != repository || !digestRE.MatchString(a.ManifestDigest) || !digestRE.MatchString(a.ConfigDigest) || a.ManifestBytes < 1 || a.ManifestBytes > maxAllowlistBytes || a.ConfigBytes < 1 || a.ConfigBytes > 16<<20 || len(a.Layers) < 1 || len(a.Layers) > 2048 {
		return a, errors.New("pull allowlist fields invalid")
	}
	seen := map[string]bool{a.ConfigDigest: true}
	var total int64
	for _, layer := range a.Layers {
		if !digestRE.MatchString(layer.Digest) || seen[layer.Digest] || layer.Bytes < 1 || layer.Bytes > 4<<30 || total > 4<<30-layer.Bytes {
			return a, errors.New("pull allowlist layer invalid")
		}
		seen[layer.Digest] = true
		total += layer.Bytes
	}
	return a, nil
}

func uniqueJSON(d *json.Decoder) error {
	token, err := d.Token()
	if err != nil {
		return err
	}
	v, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch v {
	case '{':
		seen := map[string]bool{}
		for d.More() {
			key, err := d.Token()
			if err != nil {
				return err
			}
			name, ok := key.(string)
			if !ok || seen[name] {
				return errors.New("duplicate pull allowlist key")
			}
			seen[name] = true
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	case '[':
		for d.More() {
			if err := uniqueJSON(d); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid pull allowlist delimiter")
	}
	_, err = d.Token()
	return err
}

func readAllowlist(file, expectedHash string) (allowlist, error) {
	var empty allowlist
	info, err := os.Lstat(file)
	if err != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > maxAllowlistBytes {
		return empty, errors.New("pull allowlist file unavailable")
	}
	f, err := os.Open(file)
	if err != nil {
		return empty, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return empty, errors.New("pull allowlist file changed")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxAllowlistBytes+1))
	if err != nil {
		return empty, err
	}
	return decodeAllowlist(data, expectedHash)
}

type gate struct {
	allowed map[string]int64
	client  *http.Client
}

func newGate(a allowlist, client *http.Client) *gate {
	allowed := map[string]int64{
		"/v2/" + repository + "/manifests/" + a.ManifestDigest: a.ManifestBytes,
		"/v2/" + repository + "/blobs/" + a.ConfigDigest:       a.ConfigBytes,
	}
	for _, layer := range a.Layers {
		allowed["/v2/"+repository+"/blobs/"+layer.Digest] = layer.Bytes
	}
	return &gate{allowed: allowed, client: client}
}

func (g *gate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "read-only runtime registry", http.StatusMethodNotAllowed)
		return
	}
	host, port, hostErr := net.SplitHostPort(r.Host)
	parsedPort, portErr := strconv.Atoi(port)
	if hostErr != nil || portErr != nil || host != "127.0.0.1" || parsedPort < 1024 || parsedPort > 65535 || r.ContentLength > 0 {
		http.Error(w, "runtime registry host invalid", http.StatusNotFound)
		return
	}
	if r.URL.RawQuery != "" || r.URL.Fragment != "" || r.URL.RawPath != "" || r.URL.EscapedPath() != r.URL.Path || strings.Contains(r.RequestURI, "?") || strings.Contains(r.RequestURI, "#") || r.URL.IsAbs() {
		http.Error(w, "runtime registry path invalid", http.StatusNotFound)
		return
	}
	if r.URL.Path == "/v2/" {
		w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = io.WriteString(w, "{}")
		}
		return
	}
	limit, ok := g.allowed[r.URL.Path]
	if !ok || r.Header.Get("Authorization") != "" || r.Header.Get("Proxy-Authorization") != "" {
		http.Error(w, "runtime registry object unavailable", http.StatusNotFound)
		return
	}
	start := int64(0)
	if rangeValue := r.Header.Get("Range"); rangeValue != "" {
		if !rangeRE.MatchString(rangeValue) {
			http.Error(w, "invalid range", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		var err error
		start, err = strconv.ParseInt(strings.TrimSuffix(strings.TrimPrefix(rangeValue, "bytes="), "-"), 10, 64)
		if err != nil || start >= limit {
			http.Error(w, "range exceeds object", http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, "http://"+backendAddress+r.URL.Path, nil)
	if err != nil {
		http.Error(w, "backend request invalid", http.StatusBadGateway)
		return
	}
	request.Host = backendAddress
	if start != 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	}
	response, err := g.client.Do(request)
	if err != nil {
		http.Error(w, "owned registry unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	expectedStatus := http.StatusOK
	if start != 0 {
		expectedStatus = http.StatusPartialContent
	}
	wantDigest := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if response.StatusCode != expectedStatus || response.Header.Get("Location") != "" || response.ContentLength != limit-start || response.Header.Get("Docker-Content-Digest") != "" && response.Header.Get("Docker-Content-Digest") != wantDigest {
		http.Error(w, "registry object changed", http.StatusBadGateway)
		return
	}
	if start != 0 && response.Header.Get("Content-Range") != fmt.Sprintf("bytes %d-%d/%d", start, limit-1, limit) {
		http.Error(w, "registry range changed", http.StatusBadGateway)
		return
	}
	for _, name := range []string{"Content-Type", "Content-Length", "Docker-Content-Digest", "ETag", "Content-Range", "Accept-Ranges"} {
		if value := response.Header.Get(name); value != "" {
			w.Header().Set(name, value)
		}
	}
	w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
	w.WriteHeader(expectedStatus)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.CopyN(w, response.Body, limit-start)
}

func registryConfig() string {
	return "version: 0.1\nlog:\n  level: warn\nstorage:\n  filesystem:\n    rootdirectory: " + storeDirectory + "\n  maintenance:\n    readonly:\n      enabled: true\n    uploadpurging:\n      enabled: false\n      age: 168h\n      interval: 24h\n      dryrun: false\nhttp:\n  addr: " + backendAddress + "\n"
}

func run(ctx context.Context, a allowlist) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(configPath, []byte(registryConfig()), 0600); err != nil {
		return err
	}
	child := exec.Command(registryBinary, "serve", configPath)
	child.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin", "GOMEMLIMIT=96MiB", "GOMAXPROCS=1"}
	child.Stdout, child.Stderr = os.Stderr, os.Stderr
	if err := child.Start(); err != nil {
		return err
	}
	childDone := make(chan struct{})
	var childErr error
	go func() { childErr = child.Wait(); close(childDone) }()
	defer func() {
		_ = child.Process.Signal(syscall.SIGTERM)
		select {
		case <-childDone:
		case <-time.After(5 * time.Second):
			_ = child.Process.Kill()
			<-childDone
		}
	}()
	backend := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 2 * time.Second}).DialContext}}
	ready := false
	for attempt := 0; attempt < 50; attempt++ {
		request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+backendAddress+"/v2/", nil)
		response, err := backend.Do(request)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-childDone:
			return fmt.Errorf("registry child exited before readiness: %v", childErr)
		case <-time.After(100 * time.Millisecond):
		}
	}
	if !ready {
		return errors.New("registry child did not become ready")
	}
	server := &http.Server{Addr: frontAddress, Handler: newGate(a, backend), ReadHeaderTimeout: 5 * time.Second}
	listener, err := net.Listen("tcp", frontAddress)
	if err != nil {
		return err
	}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
	case <-childDone:
		return fmt.Errorf("registry child exited after readiness: %v", childErr)
	case err := <-served:
		return err
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("luci-registry-gate 1")
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "runtime pull gate takes no arguments")
		os.Exit(2)
	}
	if os.Getenv("GOMEMLIMIT") != "32MiB" || os.Getenv("GOMAXPROCS") != "1" {
		fmt.Fprintln(os.Stderr, "runtime pull gate memory or CPU limit unavailable")
		os.Exit(1)
	}
	a, err := readAllowlist(allowlistPath, os.Getenv("LUCI_PULL_ALLOWLIST_SHA256"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	if err := run(ctx, a); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
