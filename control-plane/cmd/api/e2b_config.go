package main

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// These are operator-reviewed identities and file paths. Secrets stay in
// separate private files, never in JSON task input, Git or provider metadata.
type e2bOperatorConfig struct {
	Endpoint                string            `json:"endpoint"`
	APIKeyFile              string            `json:"apiKeyFile"`
	AttachKeyFile           string            `json:"attachKeyFile"`
	RootCAFile              string            `json:"rootCaFile,omitempty"`
	ControllerStateDir      string            `json:"controllerStateDir"`
	StagingDir              string            `json:"stagingDir"`
	EnvdOriginTemplate      string            `json:"envdOriginTemplate"`
	SandboxOriginTemplate   string            `json:"sandboxOriginTemplate"`
	Deployment              e2bbe.Deployment  `json:"deployment"`
	Template                e2bbe.Template    `json:"template"`
	Limits                  e2bbe.Limits      `json:"limits"`
	Store                   e2bbe.StoreConfig `json:"store"`
	TaskMaxSeconds          int               `json:"taskMaxSeconds"`
	TaskEstimatedCostMicros int64             `json:"taskEstimatedCostMicros"`
	ModelEgressDomains      []string          `json:"modelEgressDomains"`
}

func readPrivateE2BFile(path string, max int64) ([]byte, error) {
	if !filepath.IsAbs(path) || max < 1 {
		return nil, errors.New("cloud configuration requires absolute private file paths")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 || info.Size() < 1 || info.Size() > max {
		return nil, errors.New("cloud configuration file is missing, linked, public or oversized")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() || opened.Size() > max {
		return nil, errors.New("cloud configuration file changed while opening")
	}
	data, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || len(data) == 0 || int64(len(data)) > max {
		return nil, errors.New("cloud configuration file changed while reading")
	}
	return data, nil
}

func validE2BOriginTemplate(value string) bool {
	if strings.Count(value, "{sandboxID}") != 1 {
		return false
	}
	u, err := url.Parse(strings.Replace(value, "{sandboxID}", "verified-sandbox", 1))
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil &&
		u.Port() != "5008" && (u.Path == "" || u.Path == "/") && u.RawQuery == "" && u.Fragment == ""
}

// These effects inspect or retire an already owned attempt. They never
// authorize a new VM, upload fresh input, start an upstream agent or import
// source. Revoking a project grant must not make prior evidence or VM cleanup
// impossible; exact queue/attempt ownership is still required below.
func cloudOwnershipOnlyEffect(action string) bool {
	switch action {
	case "cancel", "observe", "guest-transport", "observe-launch", "observe-native-exit", "capture-output", "review-output":
		return true
	}
	return false
}

// openE2BTaskRuntime refuses an unconfigured or unverified cloud capability.
// The artifact store must be on the durable controller host, not on the
// replaceable Embed runtime host. The signed separation/restore receipts and
// private authenticated TLS endpoint remain operator prerequisites.
func openE2BTaskRuntime(ctx context.Context, cfg Config, projects *product.Server, authorize management.ExecutionAuthorizer) (*api.CloudTaskRuntime, func(), error) {
	if cfg.E2BConfigPath == "" {
		return nil, nil, nil
	}
	if !cfg.Worker || runtime.GOOS != "linux" || projects == nil || authorize == nil {
		return nil, nil, errors.New("cloud microVM execution requires an authenticated managed Linux worker")
	}
	data, err := readPrivateE2BFile(cfg.E2BConfigPath, 16<<10)
	if err != nil {
		return nil, nil, err
	}
	var selected e2bOperatorConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&selected) != nil || decoder.Decode(new(any)) != io.EOF ||
		selected.Store.OwnerID == "" || selected.Store.RuntimeScope != selected.Deployment.Scope ||
		selected.Template.BuildID == "" ||
		selected.TaskMaxSeconds < 1 || selected.TaskMaxSeconds > selected.Limits.MaxTaskSeconds ||
		selected.TaskEstimatedCostMicros < 1 || selected.TaskEstimatedCostMicros > selected.Limits.EstimatedCostMicros ||
		len(selected.ModelEgressDomains) == 0 || !e2bbe.ValidEgressDomains(selected.ModelEgressDomains) ||
		!validE2BOriginTemplate(selected.EnvdOriginTemplate) || !validE2BOriginTemplate(selected.SandboxOriginTemplate) ||
		!filepath.IsAbs(selected.ControllerStateDir) || !filepath.IsAbs(selected.StagingDir) {
		return nil, nil, errors.New("cloud microVM configuration has incomplete scope, limits, storage or trusted guest origins")
	}
	apiKey, err := readPrivateE2BFile(selected.APIKeyFile, 4096)
	if err != nil {
		return nil, nil, err
	}
	attachRaw, err := readPrivateE2BFile(selected.AttachKeyFile, 128)
	if err != nil {
		return nil, nil, err
	}
	attachKey, err := hex.DecodeString(strings.TrimSpace(string(attachRaw)))
	if err != nil || len(attachKey) != 32 {
		return nil, nil, errors.New("cloud task attach key must be a private 32-byte hex key")
	}
	var roots *x509.CertPool
	if selected.RootCAFile != "" {
		pemBytes, readErr := readPrivateE2BFile(selected.RootCAFile, 1<<20)
		if readErr != nil {
			return nil, nil, readErr
		}
		roots, err = x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pemBytes) {
			return nil, nil, errors.New("cloud TLS root file has no valid certificate")
		}
	}
	store, err := e2bbe.NewDirectoryStore(selected.Store)
	if err != nil {
		return nil, nil, fmt.Errorf("open durable cloud task artifacts: %w", err)
	}
	closeStore := func() { _ = store.Close() }
	var controller *e2bbe.Controller
	authorizeEffect := func(effectCtx context.Context, request e2bbe.Request, action string) error {
		i := request.Identity
		if i.OwnerID != selected.Store.OwnerID || i.RunID == "" {
			return management.ErrExecutionAuthority
		}
		scope := product.ProjectSwarmMicroVMImportScope{ProjectID: i.ProjectID, RunID: i.RunID, OperationID: i.AttemptID, ChildID: i.OperationID, OwnerID: i.OwnerID}
		if cloudOwnershipOnlyEffect(action) {
			// A completed or revoked attempt must remain observable and
			// removable. All provider and guest operations still validate the
			// immutable launch/attempt; product source import reauthorizes.
			_, err := projects.CloudSwarmCleanupOwner(scope)
			return err
		}
		if action != "allocate" && action != "stage-input" && action != "start" {
			return management.ErrExecutionAuthority
		}
		enabled, enabledErr := projects.SwarmSchedulerDispatchEnabled()
		if enabledErr != nil || !enabled {
			return management.ErrExecutionAuthority
		}
		actor, err := projects.CloudSwarmQueueOwner(scope)
		if err != nil {
			return err
		}
		run, err := projects.GetProjectSwarmRun(i.ProjectID, i.RunID)
		if err != nil || run.State == "cancelled" || run.Input.Route.ExecutionLocation != "cloud-microvm" {
			return management.ErrExecutionAuthority
		}
		if actor.UserID == "" || actor.UserID != request.BillingUserID || actor.TeamID != request.BillingTeamID || actor.Local {
			return management.ErrExecutionAuthority
		}
		current, err := authorize(effectCtx, actor, run.Input.Route.Harness, true)
		if err != nil || !current.Policy.AllowCloudProvisioning || current.Policy.MonthlyBudgetUSD <= 0 ||
			float64(request.EstimatedCostMicros) > current.Policy.MonthlyBudgetUSD*1_000_000 {
			return management.ErrExecutionAuthority
		}
		if action == "allocate" {
			if controller == nil {
				return management.ErrExecutionAuthority
			}
			rows, _ := controller.Snapshot()
			limit := int64(math.Floor(current.Policy.MonthlyBudgetUSD * 1_000_000))
			if !cloudActorMonthlyCostFits(rows, request, time.Now().UTC(), limit) {
				return management.ErrExecutionAuthority
			}
		}
		return nil
	}
	controllerConfig := e2bbe.Config{Endpoint: selected.Endpoint, APIKey: strings.TrimSpace(string(apiKey)), RootCAs: roots,
		StateDir: selected.ControllerStateDir, OwnerID: selected.Store.OwnerID, Deployment: selected.Deployment,
		Template: selected.Template, Limits: selected.Limits, EgressAllow: selected.ModelEgressDomains, Checkpoint: store, Authorize: authorizeEffect,
		GuestOrigins: func(id string) (string, string, string, error) {
			if id == "" || strings.ContainsAny(id, "/\\.:@") {
				return "", "", "", e2bbe.ErrConflict
			}
			return strings.ReplaceAll(selected.EnvdOriginTemplate, "{sandboxID}", id),
				strings.ReplaceAll(selected.SandboxOriginTemplate, "{sandboxID}", id), e2bbe.EnvdVersion, nil
		}}
	journal := filepath.Join(selected.ControllerStateDir, "attempts.json")
	if _, statErr := os.Lstat(journal); errors.Is(statErr, os.ErrNotExist) {
		if head, headErr := store.Head(ctx); headErr == nil {
			controller, err = store.RestoreController(ctx, controllerConfig, head)
		} else if errors.Is(headErr, os.ErrNotExist) {
			controller, err = e2bbe.New(controllerConfig)
		} else {
			err = headErr
		}
	} else if statErr == nil {
		controller, err = e2bbe.New(controllerConfig)
	} else {
		err = statErr
	}
	if err != nil {
		closeStore()
		return nil, nil, fmt.Errorf("restore exact cloud VM ownership: %w", err)
	}
	if err = controller.VerifyTemplateBuild(ctx); err != nil {
		_ = controller.Close(context.WithoutCancel(ctx))
		closeStore()
		return nil, nil, fmt.Errorf("verify selected cloud microVM template build: %w", err)
	}
	cloud := &api.CloudTaskRuntime{Runtime: &e2bbe.TaskRuntime{Controller: controller, Store: store},
		AttachKey: attachKey, MaxSeconds: selected.TaskMaxSeconds,
		EstimatedCostMicros: selected.TaskEstimatedCostMicros, StagingDir: selected.StagingDir,
		ModelEgressDomains: append([]string{}, selected.ModelEgressDomains...)}
	return cloud, func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = controller.Close(closeCtx)
		closeStore()
	}, nil
}
