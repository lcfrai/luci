package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// These values cross only the control plane's private, token-bound worker
// connection. Neither a browser nor a caller-selected session can request a
// pressure stop. The owning worker checks the row and durable lease again.
type HostPressureCandidate struct {
	SessionID  string    `json:"sessionId"`
	SandboxID  string    `json:"sandboxId"`
	SandboxRef string    `json:"sandboxRef"`
	LeaseID    string    `json:"leaseId"`
	CreatedAt  time.Time `json:"createdAt"`
}
type HostPressureObservation struct {
	ReserveBytes uint64                 `json:"reserveBytes"`
	Candidate    *HostPressureCandidate `json:"candidate,omitempty"`
}

const hostPressurePath = "/v1/internal/host-pressure"

type hostPressureState struct {
	below, healthy int
	acted          bool
}

type hostPressureEpisode struct {
	Version   int                    `json:"version"`
	Acted     bool                   `json:"acted"`
	Cause     string                 `json:"cause,omitempty"`
	OwnerKey  string                 `json:"ownerKey,omitempty"`
	Candidate *HostPressureCandidate `json:"candidate,omitempty"`
	At        time.Time              `json:"at,omitempty"`
}

func (g *Gateway) pressureEpisodePath() string {
	return filepath.Join(g.dir, "host-pressure-episode.json")
}

func (g *Gateway) savePressureEpisode(episode hostPressureEpisode) error {
	episode.Version = 1
	if episode.Acted {
		episode.Cause = "host_memory_pressure"
	}
	data, err := json.Marshal(episode)
	if err != nil {
		return err
	}
	return atomicPrivate(g.pressureEpisodePath(), data)
}

func (g *Gateway) loadPressureEpisode() error {
	info, err := os.Lstat(g.pressureEpisodePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 4096 {
		return errors.New("retained host pressure intent is not a bounded ordinary file")
	}
	f, err := os.Open(g.pressureEpisodePath())
	if err != nil {
		return err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return errors.New("retained host pressure intent changed during read")
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(data) > 4096 {
		return errors.New("retained host pressure intent exceeded its bound")
	}
	var episode hostPressureEpisode
	if json.Unmarshal(data, &episode) != nil || episode.Version != 1 || episode.Acted && (episode.Cause != "host_memory_pressure" || episode.Candidate == nil || episode.Candidate.SessionID == "" || episode.Candidate.LeaseID == "") {
		return errors.New("retained host pressure intent cannot be verified")
	}
	g.hostPressure.acted = episode.Acted
	return nil
}

type hostPressureRuntime struct {
	key, address, secret, teamID string
	user                         User
	policy                       Policy
}

func (g *Gateway) pressureRuntimes() []hostPressureRuntime {
	g.mu.Lock()
	defer g.mu.Unlock()
	keys := make([]string, 0, len(g.runtimes))
	for key := range g.runtimes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]hostPressureRuntime, 0, len(keys))
	for _, key := range keys {
		rt := g.runtimes[key]
		rt.mu.Lock()
		if rt.status.Status == "running" && validWorkerAddress(rt.address) && rt.secret != "" {
			out = append(out, hostPressureRuntime{key: key, address: rt.address, secret: rt.secret, teamID: rt.teamID, user: rt.user, policy: rt.policy})
		}
		rt.mu.Unlock()
	}
	return out
}

func pressureWorkerRequest(ctx context.Context, rt hostPressureRuntime, method string, total uint64, candidate *HostPressureCandidate) (HostPressureObservation, bool, error) {
	var result HostPressureObservation
	if !validWorkerAddress(rt.address) {
		return result, false, errors.New("pressure worker address changed")
	}
	timeout := 750 * time.Millisecond
	if candidate != nil {
		timeout = 12 * time.Second
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var body io.Reader
	if candidate != nil {
		data, err := json.Marshal(candidate)
		if err != nil {
			return result, false, err
		}
		body = bytes.NewReader(data)
	}
	endpoint := rt.address + hostPressurePath
	if method == http.MethodGet {
		endpoint += "?total=" + url.QueryEscape(strconv.FormatUint(total, 10))
	}
	req, err := http.NewRequestWithContext(requestCtx, method, endpoint, body)
	if err != nil {
		return result, false, err
	}
	setWorkerHeaders(req, rt.user, rt.policy, rt.secret)
	req.Header.Set("X-Luci-Host-Pressure-Control", rt.secret)
	if rt.teamID != "" {
		req.Header.Set(teamWorkerHeader, rt.teamID)
	}
	if candidate != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: timeout, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return result, candidate != nil, err // A lost stop response is uncertain.
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil || len(data) > 4096 || response.StatusCode != http.StatusOK {
		return result, candidate != nil, fmt.Errorf("pressure worker returned %d or an invalid bounded receipt", response.StatusCode)
	}
	if candidate == nil {
		if json.Unmarshal(data, &result) != nil || result.ReserveBytes == 0 {
			return result, false, errors.New("pressure worker observation is invalid")
		}
		return result, false, nil
	}
	var stopped struct {
		Acted bool   `json:"acted"`
		Error string `json:"error,omitempty"`
	}
	if json.Unmarshal(data, &stopped) != nil {
		return result, true, errors.New("pressure worker stop receipt is invalid")
	}
	if stopped.Error != "" {
		return result, stopped.Acted, errors.New("pressure worker retained its exact stop outcome")
	}
	return result, stopped.Acted, nil
}

func (g *Gateway) pressureObservations(ctx context.Context, total uint64) (uint64, *HostPressureCandidate, *hostPressureRuntime, error) {
	if g.opts.HostPressureInitialObservation == nil {
		return 0, nil, nil, errors.New("initial owner's pressure observation is unavailable")
	}
	initial, err := g.opts.HostPressureInitialObservation(ctx, total)
	if err != nil || initial.ReserveBytes == 0 {
		return 0, nil, nil, errors.New("initial owner's pressure policy or ownership is unavailable")
	}
	reserve, chosen := uint64(0), initial.Candidate
	if chosen != nil {
		reserve = initial.ReserveBytes
	}
	var owner *hostPressureRuntime
	runtimes := g.pressureRuntimes()
	for _, rt := range runtimes {
		observation, _, err := pressureWorkerRequest(ctx, rt, http.MethodGet, total, nil)
		if err != nil {
			return 0, nil, nil, err // Never pick a global newest while a worker is unreadable.
		}
		if observation.Candidate != nil {
			reserve = max(reserve, observation.ReserveBytes)
		}
		ownerKey := ""
		if owner != nil {
			ownerKey = owner.key
		}
		if observation.Candidate != nil && (chosen == nil || observation.Candidate.CreatedAt.After(chosen.CreatedAt) || observation.Candidate.CreatedAt.Equal(chosen.CreatedAt) && rt.key > ownerKey) {
			copy := rt
			chosen, owner = observation.Candidate, &copy
		}
	}
	current := g.pressureRuntimes()
	if len(current) != len(runtimes) {
		return 0, nil, nil, errors.New("pressure registry changed during global selection")
	}
	for i := range current {
		if current[i].key != runtimes[i].key || current[i].secret != runtimes[i].secret || current[i].address != runtimes[i].address {
			return 0, nil, nil, errors.New("pressure worker ownership changed during global selection")
		}
	}
	return reserve, chosen, owner, nil
}

func (g *Gateway) pressureStep(ctx context.Context, sample resourcebudget.HostSnapshot) {
	if !sample.Fresh(time.Now().UTC(), 2*time.Second) || sample.Memory.EffectiveAvailableBytes == nil || sample.Memory.EffectiveTotalBytes == nil {
		g.hostPressure.below = 0
		g.hostPressure.healthy = 0
		return
	}
	reserve, candidate, owner, err := g.pressureObservations(ctx, *sample.Memory.EffectiveTotalBytes)
	if err != nil {
		g.hostPressure.below = 0
		g.hostPressure.healthy = 0
		if g.opts.HostPressureLog != nil {
			g.opts.HostPressureLog(err)
		}
		return
	}
	if !sample.Fresh(time.Now().UTC(), 2*time.Second) {
		g.hostPressure.below = 0
		g.hostPressure.healthy = 0
		return
	}
	available := *sample.Memory.EffectiveAvailableBytes
	if available >= reserve {
		g.hostPressure.below = 0
		if available >= reserve+(256<<20) {
			g.hostPressure.healthy++
		} else {
			g.hostPressure.healthy = 0
		}
		if g.hostPressure.healthy >= 8 && g.hostPressure.acted {
			if err := g.savePressureEpisode(hostPressureEpisode{}); err != nil {
				if g.opts.HostPressureLog != nil {
					g.opts.HostPressureLog(err)
				}
				return
			}
			g.hostPressure.acted = false
		}
		return
	}
	g.hostPressure.healthy = 0
	g.hostPressure.below++
	if g.hostPressure.below < 2 || g.hostPressure.acted || candidate == nil {
		return
	}
	ownerKey := ""
	if owner != nil {
		ownerKey = owner.key
	}
	if err := g.savePressureEpisode(hostPressureEpisode{Acted: true, OwnerKey: ownerKey, Candidate: candidate, At: time.Now().UTC()}); err != nil {
		if g.opts.HostPressureLog != nil {
			g.opts.HostPressureLog(err)
		}
		return // Never perform an effect without the durable one-stop fence.
	}
	g.hostPressure.acted = true
	acted := false
	if owner == nil {
		if g.opts.HostPressureInitialStop == nil {
			return
		}
		acted, err = g.opts.HostPressureInitialStop(ctx, *candidate)
	} else {
		// Reuse only the exact active runtime selected above. A replaced worker
		// cannot inherit an old observation or receive its stop request.
		for _, rt := range g.pressureRuntimes() {
			if rt.key == owner.key && rt.secret == owner.secret && rt.address == owner.address {
				_, acted, err = pressureWorkerRequest(ctx, rt, http.MethodPost, 0, candidate)
				break
			}
		}
	}
	if acted {
		if g.opts.HostPressureLog != nil {
			g.opts.HostPressureLog(fmt.Errorf("host reserve breached (%d available, %d reserved); one exact owned session stop attempted; unknown effects remain held", available, reserve))
		}
	}
	if err != nil && g.opts.HostPressureLog != nil {
		g.opts.HostPressureLog(err)
	}
}

// RunLocalHostPressureGuard is the one host-wide pressure loop. Managed workers
// do not run their own loop; only their controller may select one of them.
func (g *Gateway) RunLocalHostPressureGuard(ctx context.Context) {
	if err := g.loadPressureEpisode(); err != nil {
		if g.opts.HostPressureLog != nil {
			g.opts.HostPressureLog(err)
		}
		return // Bad recovery state cannot authorize another stop.
	}
	probe := resourcebudget.NewHostProbe(resourcebudget.ProbeOptions{})
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			g.pressureStep(ctx, probe.Sample(ctx))
		}
	}
}
