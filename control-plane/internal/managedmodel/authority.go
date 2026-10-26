// Package managedmodel binds a saved model reference to its parent-owned
// service. It never starts a provider or grants execution permission.
package managedmodel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

type Reference struct {
	Revision int `json:"revision"`
}

// Binding is private session ownership, never a caller-supplied CID or token.
type Binding struct {
	AssistanceJobID string `json:"assistanceJobId,omitempty"`
	Boot            string `json:"boot"`
	Generation      string `json:"generation"`
	Revision        int    `json:"revision"`
	RuntimeScope    string `json:"runtimeScope"`
	LeaseID         string `json:"leaseId"`
	Endpoint        string `json:"endpoint"`
	Model           string `json:"model"`
}

func ValidBinding(b *Binding) bool {
	if b == nil {
		return true
	}
	generation, e := strconv.ParseUint(b.Generation, 10, 64)
	if e != nil || generation == 0 || strconv.FormatUint(generation, 10) != b.Generation || b.Revision < 1 || len(b.Boot) != 64 || strings.Trim(b.Boot, "0123456789abcdef") != "" || len(b.RuntimeScope) != 64 || strings.Trim(b.RuntimeScope, "0123456789abcdef") != "" || b.LeaseID == "" || len(b.LeaseID) > 200 || strings.ContainsAny(b.LeaseID, "\x00\r\n\t \\/") || b.Model == "" || len(b.Model) > 200 {
		return false
	}
	u, e := url.Parse(b.Endpoint)
	if e != nil || u.Scheme != "http" || u.Path != "/v1" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawPath != "" {
		return false
	}
	p, e := strconv.Atoi(u.Port())
	if e != nil || p < 1 || p > 65535 {
		return false
	}
	switch u.Hostname() {
	case "127.0.0.1", "localhost", "host.docker.internal", "::1":
		return true
	}
	return false
}

type Profile struct {
	ContextTokens    int    `json:"contextTokens"`
	KVCacheType      string `json:"kvCacheType"`
	FlashAttention   bool   `json:"flashAttention"`
	Parallelism      int    `json:"parallelism"`
	JobMemoryMiB     int    `json:"jobMemoryMiB"`
	ProcessMemoryMiB int    `json:"processMemoryMiB"`
	CPULimitPercent  int    `json:"cpuLimitPercent"`
}

func FixedProfile() Profile { return Profile{16384, "q8_0", true, 1, 7168, 6656, 20} }

type Failure struct {
	Code         string                  `json:"code"`
	ExitCode     *uint32                 `json:"exitCode,omitempty"`
	HostCapacity *HostCapacityDiagnostic `json:"hostCapacity,omitempty"`
}

// Fixed reasons and bounded numeric observations only; never host paths,
// environment, vendor output or an inferred cause when evidence is missing.
type HostCapacityDiagnostic struct {
	Phase                  string   `json:"phase"`
	Reason                 string   `json:"reason"`
	PhysicalAvailableBytes *uint64  `json:"physicalAvailableBytes"`
	CommitAvailableBytes   *uint64  `json:"commitAvailableBytes"`
	CPUPercent             *float64 `json:"cpuPercent"`
	SampleAgeMillis        *int64   `json:"sampleAgeMillis"`
}
type Status struct {
	State      string    `json:"state"`
	Generation string    `json:"generation"`
	ObservedAt time.Time `json:"observedAt"`
	Endpoint   string    `json:"endpoint,omitempty"`
	Model      string    `json:"model,omitempty"`
	Failure    *Failure  `json:"failure,omitempty"`
}

// Observation is private parent authority, not a browser request or readiness
// cache that a worker can promote. The separate lock survives atomic replaces.
type Observation struct {
	Version     int    `json:"version"`
	Boot        string `json:"boot"`
	Revision    int    `json:"revision"`
	Withdrawing bool   `json:"withdrawing"`
	Status      Status `json:"status"`
}
type authority struct{ Path, Boot string }

var configured atomic.Pointer[authority]

func Configure(path, boot string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(boot) != 64 || strings.Trim(boot, "0123456789abcdef") != "" {
		return errors.New("managed model authority is invalid")
	}
	next := &authority{path, boot}
	old := configured.Load()
	if old != nil && *old != *next {
		return errors.New("managed model authority is already bound")
	}
	configured.Store(next)
	return nil
}
func ConfigureFromEnvironment() error {
	p, b := os.Getenv("LUCI_MANAGED_MODEL_STATUS_FILE"), os.Getenv("LUCI_MANAGED_MODEL_BOOT")
	if p == "" && b == "" {
		return nil
	}
	return Configure(p, b)
}
func ReleaseAuthority(path, boot string) {
	old := configured.Load()
	if old != nil && old.Path == path && old.Boot == boot {
		configured.CompareAndSwap(old, nil)
	}
}
func Read(path string) (Observation, error) {
	var o Observation
	if err := hostpath.OneName(path); err != nil {
		return o, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 8192 {
		return o, errors.New("managed model observation unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return o, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return o, errors.New("managed model observation changed")
	}
	raw, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		return o, errors.New("managed model observation exceeds bound")
	}
	after, err := f.Stat()
	if err != nil || opened.Size() != after.Size() || !opened.ModTime().Equal(after.ModTime()) {
		return o, errors.New("managed model observation changed")
	}
	if json.Unmarshal(raw, &o) != nil || o.Version != 1 {
		return o, errors.New("managed model observation invalid")
	}
	return o, nil
}
func Check(ref *Reference, endpoint, model string) (Observation, error) {
	var o Observation
	if ref == nil {
		return o, nil
	}
	a := configured.Load()
	if a == nil {
		return o, errors.New("The managed model is unavailable in this workspace.")
	}
	o, err := Read(a.Path)
	if err != nil {
		return o, errors.New("The managed model status is unavailable. Open its settings before trying again.")
	}
	age := time.Since(o.Status.ObservedAt)
	u, e := url.Parse(endpoint)
	if e != nil {
		return o, errors.New("The managed model connection changed.")
	}
	// Docker rewrites only the hostname; port, protocol and exact model stay bound.
	host := u.Hostname()
	if host != "127.0.0.1" && host != "localhost" && host != "host.docker.internal" && host != "::1" {
		return o, errors.New("The managed model endpoint differs.")
	}
	u.Host = "127.0.0.1:" + u.Port()
	generation, e := strconv.ParseUint(o.Status.Generation, 10, 64)
	if ref.Revision < 1 || o.Boot != a.Boot || o.Revision != ref.Revision || o.Withdrawing || age < 0 || age > 5*time.Second || (o.Status.State != "available" && o.Status.State != "loaded") || e != nil || generation == 0 || strconv.FormatUint(generation, 10) != o.Status.Generation || u.String() != o.Status.Endpoint || model != o.Status.Model {
		return o, errors.New("The managed model is stopped, changing or no longer matches this connection. Refresh its settings.")
	}
	return o, nil
}

// Hold pins the current generation across an existing execution admission and
// Backend.Create. Stop first withdraws publication, then takes the exclusive
// barrier before inspecting/retiring the existing owned session ledger.
func Hold(ctx context.Context, ref *Reference, endpoint, model string) (func(), Observation, error) {
	if ref == nil {
		return func() {}, Observation{}, nil
	}
	a := configured.Load()
	if a == nil {
		return nil, Observation{}, errors.New("managed model authority unavailable")
	}
	unlock, err := lock(ctx, a.Path+".barrier", false)
	if err != nil {
		return nil, Observation{}, err
	}
	o, err := Check(ref, endpoint, model)
	if err != nil {
		unlock()
		return nil, o, err
	}
	return unlock, o, nil
}
func WithdrawBarrier(ctx context.Context, path string) (func(), error) {
	return lock(ctx, path+".barrier", true)
}
