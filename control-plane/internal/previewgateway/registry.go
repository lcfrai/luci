// Package previewgateway exposes owned Docker applications on separate browser
// origins. Registration is private filesystem state, never a public HTTP API.
package previewgateway

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

var name = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)
var capability = regexp.MustCompile(`^[a-f0-9]{64}$`)
var capabilityHostname = regexp.MustCompile(`^[a-f0-9]{32}\.[a-f0-9]{32}\.localhost$`)

// CapabilityHostname preserves the full private registration key in two DNS
// labels. A single 64-character label cannot resolve in real browsers because
// the DNS label limit is 63; existing persisted keys remain unchanged.
func CapabilityHostname(key string) string {
	if !capability.MatchString(key) {
		return ""
	}
	return key[:32] + "." + key[32:] + ".localhost"
}

func IsCapabilityHostname(host string) bool { return capabilityHostname.MatchString(host) }

type Target struct {
	Kind           string            `json:"kind"`
	ProjectID      string            `json:"projectId"`
	ResourceID     string            `json:"resourceId"`
	Upstream       string            `json:"upstream"`
	Container      string            `json:"container,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
	PublishableKey string            `json:"publishableKey,omitempty"`
	BridgeKey      string            `json:"bridgeKey,omitempty"`
	ClientKind     string            `json:"clientKind,omitempty"`
}
type record struct {
	Target
	Key string `json:"key"`
}
type Registry struct {
	dir    string
	port   int
	domain string
	mu     sync.Mutex
}

func NewRegistry(stateDir string, port int) (*Registry, error) {
	return NewRegistryWithDomain(stateDir, port, "")
}

func NewRegistryWithDomain(stateDir string, port int, domain string) (*Registry, error) {
	if domain != "" {
		if err := ValidatePreviewBaseDomain(domain); err != nil {
			return nil, err
		}
	}
	if !filepath.IsAbs(stateDir) || port < 1 || port > 65535 {
		return nil, errors.New("browser gateway requires an absolute private state directory and valid port")
	}
	dir := filepath.Join(stateDir, "browser-previews")
	if _, err := hostpath.ConfineToMounts([]string{stateDir}, stateDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if _, err := hostpath.ConfineToMounts([]string{stateDir}, dir); err != nil {
		return nil, err
	}
	if err := protect(dir); err != nil {
		return nil, err
	}
	return &Registry{dir: dir, port: port, domain: domain}, nil
}
func validate(t Target, port int) (*url.URL, error) {
	if !name.MatchString(t.ProjectID) || !name.MatchString(t.ResourceID) {
		return nil, errors.New("invalid browser target identity")
	}
	if t.Kind != "application" && t.Kind != "preview" && t.Kind != "data" && t.Kind != "bridge" {
		return nil, errors.New("invalid browser target kind")
	}
	u, err := url.Parse(t.Upstream)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Hostname() != "127.0.0.1" || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("browser upstream must be a literal private loopback origin")
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p < 1 || p > 65535 || p == port || p == 8730 {
		return nil, errors.New("browser upstream cannot be the control plane or gateway")
	}
	if t.Kind != "bridge" {
		if !name.MatchString(t.Container) || len(t.Labels) < 2 || len(t.Labels) > 8 {
			return nil, errors.New("browser target requires owned Docker labels")
		}
		bound := false
		for k, v := range t.Labels {
			if len(k) > 128 || len(v) > 128 {
				return nil, errors.New("invalid ownership label")
			}
			if (k == "luci.project-id" || k == "ai.lcfr.luci.platform.project") && v == t.ProjectID {
				bound = true
			}
		}
		if !bound {
			return nil, errors.New("Docker target must bind its project label")
		}
	} else {
		if !capability.MatchString(t.BridgeKey) || (t.ClientKind != "" && t.ClientKind != "application" && t.ClientKind != "data") {
			return nil, errors.New("owned bridge requires a private liveness key and valid client kind")
		}
	}
	if (t.Kind == "data" || t.ClientKind == "data") && (len(t.PublishableKey) < 32 || len(t.PublishableKey) > 4096) {
		return nil, errors.New("data target requires its client publishable key")
	}
	if t.Kind != "bridge" && t.ClientKind != "" {
		return nil, errors.New("client kind is reserved for owned bridges")
	}
	return u, nil
}
func (r *Registry) filename(t Target) string {
	s := sha256.Sum256([]byte(t.Kind + "\x00" + t.ProjectID + "\x00" + t.ResourceID))
	return filepath.Join(r.dir, hex.EncodeToString(s[:])+".json")
}
func (r *Registry) Publish(t Target) (string, error) {
	if _, err := validate(t, r.port); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	path := r.filename(t)
	key := ""
	if old, err := readRecord(r.dir, path, r.port); err == nil && old.Kind == t.Kind && old.ProjectID == t.ProjectID && old.ResourceID == t.ResourceID {
		key = old.Key
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if key == "" {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		key = hex.EncodeToString(b)
	}
	b, err := json.Marshal(record{Target: t, Key: key})
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(r.dir, ".register-*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = protect(tmp); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if _, err = os.Lstat(path); err == nil {
		if _, err = hostpath.ConfineToMounts([]string{r.dir}, path); err != nil {
			return "", err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err = os.Rename(tmp, path); err != nil {
		return "", err
	}
	return browserOrigin(key, r.port, r.domain), nil
}
func (r *Registry) Remove(kind, project, resource string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.filename(Target{Kind: kind, ProjectID: project, ResourceID: resource})
	if _, err := os.Lstat(p); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if _, err := hostpath.ConfineToMounts([]string{r.dir}, p); err != nil {
		return err
	}
	err := os.Remove(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func readRecord(dir, path string, port int) (record, error) {
	var out record
	if _, err := hostpath.ConfineToMounts([]string{dir}, path); err != nil {
		return out, err
	}
	if err := hostpath.OneName(path); err != nil {
		return out, err
	}
	f, err := os.Open(path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil {
		return out, err
	}
	if len(b) > 16384 {
		return out, errors.New("browser registration exceeds limit")
	}
	if err = json.Unmarshal(b, &out); err != nil {
		return out, err
	}
	if !capability.MatchString(out.Key) {
		return out, errors.New("invalid browser capability")
	}
	_, err = validate(out.Target, port)
	return out, err
}
func requestKey(host string, port int) string {
	h, p, err := net.SplitHostPort(host)
	if err != nil || p != strconv.Itoa(port) {
		return ""
	}
	if !IsCapabilityHostname(h) {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSuffix(h, ".localhost"), ".", "")
}
