package api

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type nativeOpenCodeTurn struct {
	State               string `json:"state"`
	TurnID              string `json:"turnId,omitempty"`
	ToolClass           string `json:"toolClass,omitempty"`
	ExecutionGeneration string `json:"executionGeneration,omitempty"`
	LastOutputAt        string `json:"lastOutputAt,omitempty"`
}

// Explicit selected-session reads have a process-wide limit. A slow native
// service cannot occupy a worker for each row of a project or swarm listing.
var nativeTurnReads = make(chan struct{}, 2)

func (a *API) handleSessionNativeTurn(w http.ResponseWriter, r *http.Request) {
	s, ok := a.sessionByID(w, r)
	if !ok {
		return
	}
	unknown := nativeOpenCodeTurn{State: "unknown", ExecutionGeneration: sessionExecutionGeneration(s), LastOutputAt: sessionTime(s.LastOutputAt)}
	if unknown.ExecutionGeneration == "" || s.Harness != "opencode" || s.ImplementationContext == nil || !sessionAttachLive(s) || s.Backend != "docker" || s.Endpoint == "" || s.AttachToken == "" {
		writeNativeTurn(w, unknown)
		return
	}
	select {
	case nativeTurnReads <- struct{}{}:
		defer func() { <-nativeTurnReads }()
	default:
		writeNativeTurn(w, unknown)
		return
	}
	turn := observeNativeOpenCodeTurn(r.Context(), s)
	current, err := a.deps.Store.Get(s.ID)
	if err != nil || !nativeTurnSnapshotMatches(s, current) || r.Context().Err() != nil || turn == nil {
		writeNativeTurn(w, unknown)
		return
	}
	turn.ExecutionGeneration, turn.LastOutputAt = unknown.ExecutionGeneration, unknown.LastOutputAt
	writeNativeTurn(w, *turn)
}

func writeNativeTurn(w http.ResponseWriter, turn nativeOpenCodeTurn) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(turn)
}

func nativeTurnSnapshotMatches(before, after registry.Session) bool {
	return sessionExecutionGeneration(before) != "" && sessionExecutionGeneration(before) == sessionExecutionGeneration(after) &&
		before.Status == after.Status && before.LastOutputAt.Equal(after.LastOutputAt) &&
		before.Endpoint == after.Endpoint && before.AttachToken == after.AttachToken &&
		sessionAttachLive(after)
}

// The only native read is the sandboxd summary protected by the session's
// attach secret. Its endpoint comes from the exact owned Docker allocation;
// a stale/reused port has a different secret and therefore remains unknown.
// No arbitrary upstream transcript or model response enters the API.
func observeNativeOpenCodeTurn(ctx context.Context, s registry.Session) *nativeOpenCodeTurn {
	if s.Harness != "opencode" || s.ImplementationContext == nil || !sessionAttachLive(s) || s.Backend != "docker" || s.Endpoint == "" || s.AttachToken == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(s.Endpoint)
	if err != nil || host != "127.0.0.1" {
		return nil
	}
	client := &http.Client{Timeout: 750 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+s.Endpoint+"/native-turn", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+s.AttachToken)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength > 512 {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 513))
	if err != nil || len(data) > 512 {
		return nil
	}
	var turn nativeOpenCodeTurn
	if json.Unmarshal(data, &turn) != nil || turn.State != "finished" || !nativeTurnID(turn.TurnID) {
		return nil
	}
	if turn.ToolClass != "reads-only" && turn.ToolClass != "none" && turn.ToolClass != "effects-unverified" {
		return nil
	}
	return &turn
}

func nativeTurnID(id string) bool {
	if len(id) < 4 || len(id) > 128 || strings.TrimSpace(id) != id {
		return false
	}
	for _, c := range id {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}
