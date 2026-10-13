package main

import (
	"bytes"
	"context"
	"crypto/tls"
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
	Endpoint                  string                      `json:"endpoint"`
	APIKeyFile                string                      `json:"apiKeyFile"`
	AdminAPIKeyFile           string                      `json:"adminApiKeyFile"`
	HostNodeID                string                      `json:"hostNodeId"`
	AttachKeyFile             string                      `json:"attachKeyFile"`
	RootCAFile                string                      `json:"rootCaFile,omitempty"`
	ClientCertFile            string                      `json:"clientCertFile,omitempty"`
	ClientKeyFile             string                      `json:"clientKeyFile,omitempty"`
	AzureSecrets              *azureSecretMaterialization `json:"azureSecrets,omitempty"`
	AzureMount                *azureDurableMount          `json:"azureMount,omitempty"`
	ControllerStateDir        string                      `json:"controllerStateDir"`
	StagingDir                string                      `json:"stagingDir"`
	EnvdOriginTemplate        string                      `json:"envdOriginTemplate"`
	SandboxOriginTemplate     string                      `json:"sandboxOriginTemplate"`
	Deployment                e2bbe.Deployment            `json:"deployment"`
	Template                  e2bbe.Template              `json:"template"`
	TemplateCreateJournalPath string                      `json:"templateCreateJournalPath"`
	TemplateBuildJournalPath  string                      `json:"templateBuildJournalPath"`
	TemplateFromImage         string                      `json:"templateFromImage"`
	Limits                    e2bbe.Limits                `json:"limits"`
	Store                     e2bbe.StoreConfig           `json:"store"`
	TaskMaxSeconds            int                         `json:"taskMaxSeconds"`
	TaskEstimatedCostMicros   int64                       `json:"taskEstimatedCostMicros"`
	ModelEgressDomains        []string                    `json:"modelEgressDomains"`
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

func validAzureGuestOriginTemplate(value, kind, gateway string) bool {
	return value == "https://"+kind+"-{sandboxID}."+gateway && validE2BOriginTemplate(value)
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

// A provider outage must not hide already owned VM attempts from the API's
// recovery view. The restored journal is still mandatory and retains every
// charge; this exception only permits startup for an existing live/uncertain
// attempt or an absent VM whose product outcome has not been joined. New
// allocation rechecks both provider prerequisites in dispatch.
type cloudStartupController interface {
	Snapshot() ([]e2bbe.Record, e2bbe.Usage)
	VerifyTemplateBuild(context.Context) error
	VerifyNodeCapacity(context.Context) error
}

func verifyCloudStartup(ctx context.Context, controller cloudStartupController) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rows, _ := controller.Snapshot()
	retained := false
	for _, row := range rows {
		if row.State != e2bbe.Absent || row.ProductOutcome == "" {
			retained = true
			break
		}
	}
	if err := controller.VerifyTemplateBuild(ctx); err != nil && !retained {
		return fmt.Errorf("verify selected cloud microVM template build: %w", err)
	}
	if err := controller.VerifyNodeCapacity(ctx); err != nil && !errors.Is(err, e2bbe.ErrCapacity) && !retained {
		return fmt.Errorf("verify authenticated cloud microVM host: %w", err)
	}
	return ctx.Err()
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
		selected.Deployment.Provider == "azure" && selected.Store.AzureDataDiskID != selected.Deployment.AzureDataDiskID ||
		selected.Deployment.Provider != "azure" && selected.Store.AzureDataDiskID != "" ||
		selected.Deployment.Provider == "azure" && (selected.AzureSecrets == nil || selected.APIKeyFile != "" || selected.AdminAPIKeyFile != "" || selected.AttachKeyFile != "" || selected.ClientCertFile != "" || selected.ClientKeyFile != "") ||
		selected.Deployment.Provider == "azure" && selected.AzureMount == nil ||
		selected.Deployment.Provider != "azure" && selected.AzureSecrets != nil ||
		selected.Deployment.Provider != "azure" && selected.AzureMount != nil ||
		(selected.ClientCertFile == "") != (selected.ClientKeyFile == "") ||
		selected.Template.BuildID == "" || selected.HostNodeID == "" || selected.Deployment.Provider != "azure" && selected.AdminAPIKeyFile == "" ||
		selected.TaskMaxSeconds < 1 || selected.TaskMaxSeconds > selected.Limits.MaxTaskSeconds ||
		selected.TaskEstimatedCostMicros < 1 || selected.TaskEstimatedCostMicros > selected.Limits.EstimatedCostMicros ||
		len(selected.ModelEgressDomains) == 0 || !e2bbe.ValidEgressDomains(selected.ModelEgressDomains) ||
		!validE2BOriginTemplate(selected.EnvdOriginTemplate) || !validE2BOriginTemplate(selected.SandboxOriginTemplate) ||
		selected.Deployment.Provider == "azure" && (!validAzureGuestOriginTemplate(selected.EnvdOriginTemplate, "envd", selected.Deployment.AzureGatewayHost) ||
			!validAzureGuestOriginTemplate(selected.SandboxOriginTemplate, "sandbox", selected.Deployment.AzureGatewayHost)) ||
		!filepath.IsAbs(selected.ControllerStateDir) || !filepath.IsAbs(selected.StagingDir) ||
		!filepath.IsAbs(selected.TemplateCreateJournalPath) || !filepath.IsAbs(selected.TemplateBuildJournalPath) {
		return nil, nil, errors.New("cloud microVM configuration has incomplete scope, limits, storage or trusted guest origins")
	}
	if selected.AzureMount != nil {
		if err := verifyAzureDurableMount(*selected.AzureMount, selected, cfg); err != nil {
			return nil, nil, fmt.Errorf("verify Azure controller disk and restoration boundary: %w", err)
		}
	}
	if _, err = (e2bbe.TemplatePublication{TemplateID: selected.Template.ID, BuildID: selected.Template.BuildID,
		FromImage: selected.TemplateFromImage, CreateJournalPath: selected.TemplateCreateJournalPath,
		JournalPath: selected.TemplateBuildJournalPath}).ReadTemplateBuildIntent(); err != nil {
		return nil, nil, fmt.Errorf("selected cloud microVM template lacks its exact private Create/build intent: %w", err)
	}
	var azureSecretsCleanup func()
	if selected.AzureSecrets != nil {
		source, sourceErr := newAzureSecretSource(*selected.AzureSecrets)
		if sourceErr != nil {
			return nil, nil, sourceErr
		}
		files, cleanup, materializeErr := materializeAzureSecrets(ctx, *selected.AzureSecrets, source)
		if materializeErr != nil {
			return nil, nil, materializeErr
		}
		azureSecretsCleanup = cleanup
		selected.APIKeyFile, selected.AdminAPIKeyFile, selected.AttachKeyFile = files.APIKey, files.AdminKey, files.AttachKey
		selected.ClientCertFile, selected.ClientKeyFile = files.ClientCert, files.ClientKey
	}
	succeeded := false
	defer func() {
		if !succeeded && azureSecretsCleanup != nil {
			azureSecretsCleanup()
		}
	}()
	apiKey, err := readPrivateE2BFile(selected.APIKeyFile, 4096)
	if err != nil {
		return nil, nil, err
	}
	adminKey, err := readPrivateE2BFile(selected.AdminAPIKeyFile, 4096)
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
	var clientCertificate *tls.Certificate
	if selected.ClientCertFile != "" {
		certPEM, certErr := readPrivateE2BFile(selected.ClientCertFile, 1<<20)
		if certErr != nil {
			return nil, nil, certErr
		}
		keyPEM, keyErr := readPrivateE2BFile(selected.ClientKeyFile, 1<<20)
		if keyErr != nil {
			return nil, nil, keyErr
		}
		cert, pairErr := tls.X509KeyPair(certPEM, keyPEM)
		if pairErr != nil || len(cert.Certificate) == 0 {
			return nil, nil, errors.New("cloud TLS client identity is invalid")
		}
		leaf, parseErr := x509.ParseCertificate(cert.Certificate[0])
		if parseErr != nil || time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
			return nil, nil, errors.New("cloud TLS client identity is not currently valid")
		}
		cert.Leaf = leaf
		clientCertificate = &cert
	}
	store, err := e2bbe.NewDirectoryStore(selected.Store)
	if err != nil {
		return nil, nil, fmt.Errorf("open durable cloud task artifacts: %w", err)
	}
	closeStore := func() { _ = store.Close() }
	var controller *e2bbe.Controller
	authorizeEffect := func(effectCtx context.Context, request e2bbe.Request, action string) error {
		if selected.AzureMount != nil && !cloudOwnershipOnlyEffect(action) {
			if err := verifyAzureDurableMount(*selected.AzureMount, selected, cfg); err != nil {
				return management.ErrExecutionAuthority
			}
		}
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
			rows, settled := controller.SpendSnapshot()
			limit := int64(math.Floor(current.Policy.MonthlyBudgetUSD * 1_000_000))
			if !cloudActorMonthlyCostFits(rows, settled, request, time.Now().UTC(), limit) {
				return management.ErrExecutionAuthority
			}
		}
		return nil
	}
	controllerConfig := e2bbe.Config{Endpoint: selected.Endpoint, APIKey: strings.TrimSpace(string(apiKey)), AdminAPIKey: strings.TrimSpace(string(adminKey)), HostNodeID: selected.HostNodeID, RootCAs: roots, ClientCertificate: clientCertificate,
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
	if err = verifyCloudStartup(ctx, controller); err != nil {
		_ = controller.Close(context.WithoutCancel(ctx))
		closeStore()
		return nil, nil, err
	}
	cloud := &api.CloudTaskRuntime{Runtime: &e2bbe.TaskRuntime{Controller: controller, Store: store},
		AttachKey: attachKey, MaxSeconds: selected.TaskMaxSeconds,
		EstimatedCostMicros: selected.TaskEstimatedCostMicros, StagingDir: selected.StagingDir,
		ModelEgressDomains: append([]string{}, selected.ModelEgressDomains...)}
	succeeded = true
	return cloud, func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = controller.Close(closeCtx)
		closeStore()
		if azureSecretsCleanup != nil {
			azureSecretsCleanup()
		}
	}, nil
}
