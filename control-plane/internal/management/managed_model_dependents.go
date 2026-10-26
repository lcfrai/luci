package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const managedModelDependentsPath = "/v1/internal/managed-model-dependents"

func (g *Gateway) settleManagedPrivateAPIs(ctx context.Context) error {
	g.mu.Lock()
	list := make([]*userRuntime, 0, len(g.runtimes))
	for _, rt := range g.runtimes {
		list = append(list, rt)
	}
	g.mu.Unlock()
	for _, rt := range list {
		rt.stopContext(ctx)
	}
	for _, rt := range list {
		select {
		case <-rt.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}
func (g *Gateway) settleManagedModelDependents(ctx context.Context, boot, generation string, stop bool) error {
	if g.opts.ManagedModelDependents == nil {
		return errors.New("managed model initial runtime cleanup unavailable")
	}
	if err := g.opts.ManagedModelDependents(ctx, boot, generation, stop); err != nil {
		return err
	}
	for _, rt := range g.pressureRuntimes() {
		raw, _ := json.Marshal(map[string]any{"boot": boot, "generation": generation, "stop": stop})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.address+managedModelDependentsPath, bytes.NewReader(raw))
		if err != nil {
			return err
		}
		setWorkerHeaders(request, rt.user, rt.policy, rt.secret)
		request.Header.Set("X-Luci-Model-Control", rt.secret)
		request.Header.Set("Content-Type", "application/json")
		if rt.teamID != "" {
			request.Header.Set(teamWorkerHeader, rt.teamID)
		}
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		response, err := client.Do(request)
		if err != nil {
			return errors.New("managed model dependent runtime outcome unknown")
		}
		raw, err = io.ReadAll(io.LimitReader(response.Body, 129))
		closeErr := response.Body.Close()
		if err != nil || closeErr != nil || len(raw) > 128 || response.StatusCode != http.StatusOK {
			return errors.New("managed model dependent work remains or cleanup is unknown")
		}
		var out struct {
			Settled bool `json:"settled"`
		}
		if json.Unmarshal(raw, &out) != nil || !out.Settled {
			return errors.New("managed model dependent settlement unverified")
		}
	}
	return nil
}
