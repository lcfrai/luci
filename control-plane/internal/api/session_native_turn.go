package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type nativeOpenCodeTurn struct {
	State     string `json:"state"`
	TurnID    string `json:"turnId"`
	ToolClass string `json:"toolClass"`
}

// The only native read is the sandboxd summary protected by the session's
// attach secret. Its endpoint comes from the exact owned Docker allocation;
// a stale/reused port has a different secret and therefore remains unknown.
// No arbitrary upstream transcript or model response enters the API.
func observeNativeOpenCodeTurn(s registry.Session) *nativeOpenCodeTurn {
	if s.Harness != "opencode" || s.ImplementationContext == nil || !sessionAttachLive(s) || s.Backend != "docker" || s.Endpoint == "" || s.AttachToken == "" {
		return nil
	}
	host, _, err := net.SplitHostPort(s.Endpoint)
	if err != nil || host != "127.0.0.1" {
		return nil
	}
	client := &http.Client{Timeout: 750 * time.Millisecond, Transport: &http.Transport{Proxy: nil, DisableKeepAlives: true}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, err := http.NewRequest(http.MethodGet, "http://"+s.Endpoint+"/native-turn", nil)
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
