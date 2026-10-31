package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// Each file is the same existing verified host configuration. This catalogue
// grants no provisioning/acquisition capability and includes no credential
// values. Independent host state/checkpoints and ownership remain mandatory.
type e2bOperatorPool struct {
	Version       int          `json:"version"`
	Scope         string       `json:"scope"`
	StateDir      string       `json:"stateDir"`
	DefaultHostID string       `json:"defaultHostId"`
	Limits        e2bbe.Limits `json:"limits"`
	Hosts         []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		ConfigPath string `json:"configPath"`
		Draining   bool   `json:"draining"`
	} `json:"hosts"`
}

func openE2BTaskRuntime(ctx context.Context, cfg Config, projects *product.Server, authorize management.ExecutionAuthorizer) (*api.CloudTaskRuntime, func(), error) {
	if cfg.E2BConfigPath == "" {
		return nil, nil, nil
	}
	if !cfg.Worker || runtime.GOOS != "linux" || projects == nil || authorize == nil {
		return nil, nil, errors.New("cloud microVM execution requires an authenticated managed Linux worker")
	}
	raw, err := readPrivateE2BFile(cfg.E2BConfigPath, 16<<10)
	if err != nil {
		return nil, nil, err
	}
	var header map[string]json.RawMessage
	if json.Unmarshal(raw, &header) != nil {
		return nil, nil, errors.New("cloud configuration must be one bounded JSON object")
	}
	if _, poolMode := header["hosts"]; !poolMode {
		return openE2BTaskRuntimeSingle(ctx, cfg, projects, authorize, nil)
	}
	if !cfg.Worker || runtime.GOOS != "linux" || projects == nil || authorize == nil {
		return nil, nil, errors.New("cloud host pools require the authenticated managed Linux worker")
	}
	var selected e2bOperatorPool
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&selected) != nil || decoder.Decode(new(any)) != io.EOF || selected.Version != 1 || !filepath.IsAbs(selected.StateDir) || len(selected.Hosts) < 1 || len(selected.Hosts) > 8 {
		return nil, nil, errors.New("cloud host pool requires 1–8 explicit existing hosts and a private durable controller state path")
	}
	hosts := []api.CloudTaskHost{}
	cleanups := []func(){}
	seen := map[string]bool{}
	var pool *api.CloudTaskPool
	cleanup := func() {
		if pool != nil {
			_ = pool.Close()
		}
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}
	for _, entry := range selected.Hosts {
		if !filepath.IsAbs(entry.ConfigPath) || filepath.Clean(entry.ConfigPath) == filepath.Clean(cfg.E2BConfigPath) || seen[filepath.Clean(entry.ConfigPath)] {
			cleanup()
			return nil, nil, errors.New("cloud pool host files must be distinct absolute existing single-host configurations")
		}
		seen[filepath.Clean(entry.ConfigPath)] = true
		hostRaw, readErr := readPrivateE2BFile(entry.ConfigPath, 16<<10)
		if readErr != nil {
			cleanup()
			return nil, nil, readErr
		}
		var hostHeader map[string]json.RawMessage
		if json.Unmarshal(hostRaw, &hostHeader) != nil || hostHeader["hosts"] != nil {
			cleanup()
			return nil, nil, errors.New("nested cloud pools are not supported")
		}
		hostID := entry.ID
		guard := func(current context.Context, request e2bbe.Request, budget int64) error {
			if pool == nil {
				return management.ErrExecutionAuthority
			}
			return pool.AuthorizeAllocation(current, hostID, request, budget)
		}
		hostConfig := cfg
		hostConfig.E2BConfigPath = entry.ConfigPath
		host, closeHost, openErr := openE2BTaskRuntimeSingle(ctx, hostConfig, projects, authorize, guard)
		if openErr != nil {
			cleanup()
			return nil, nil, openErr
		}
		cleanups = append(cleanups, closeHost)
		// Re-read only the declared operator config, never any credential values.
		latest, readErr := readPrivateE2BFile(entry.ConfigPath, 16<<10)
		if readErr != nil || !bytes.Equal(hostRaw, latest) {
			cleanup()
			return nil, nil, errors.New("cloud host configuration changed during pool admission")
		}
		sum := sha256.Sum256(hostRaw)
		var exact e2bOperatorConfig
		if json.Unmarshal(hostRaw, &exact) != nil {
			cleanup()
			return nil, nil, errors.New("cloud host identity could not be retained")
		}
		// One verified installed node cannot be counted twice through endpoint aliases or changed owner/limit fields.
		physical, _ := json.Marshal([]string{exact.HostNodeID})
		physicalSum := sha256.Sum256(physical)
		hosts = append(hosts, api.CloudTaskHost{ID: entry.ID, Name: entry.Name, ConfigDigest: hex.EncodeToString(sum[:]), HostKey: hex.EncodeToString(physicalSum[:]), Draining: entry.Draining, Runtime: host})
	}
	pool, err = api.NewCloudTaskPool(api.CloudTaskPoolConfig{StateDir: selected.StateDir, Scope: selected.Scope, DefaultHostID: selected.DefaultHostID, Limits: selected.Limits}, hosts)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	var defaultRuntime *api.CloudTaskRuntime
	for _, host := range hosts {
		if host.ID == selected.DefaultHostID {
			defaultRuntime = host.Runtime
		}
	}
	if defaultRuntime == nil {
		cleanup()
		return nil, nil, errors.New("cloud pool default host is not configured")
	}
	result := *defaultRuntime
	result.Pool = pool
	return &result, cleanup, nil
}
