package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func openAzureElasticSingle(ctx context.Context, cfg Config, projects *product.Server, authorize management.ExecutionAuthorizer, base e2bOperatorConfig) (*api.CloudTaskRuntime, func(), error) {
	if base.Elasticity == nil || base.AzureSecrets == nil || base.AzureMount == nil || base.Deployment.Provider != "azure" {
		return nil, nil, e2bbe.ErrUnsupported
	}
	selection := *base.Elasticity
	base.Elasticity = nil
	prefix := "/subscriptions/" + base.Deployment.AzureSubscriptionID + "/resourcegroups/"
	if !strings.HasPrefix(selection.HostResourceGroupID, prefix) || strings.ContainsAny(strings.TrimPrefix(selection.HostResourceGroupID, prefix), "/\\?# ") || strings.TrimPrefix(selection.HostResourceGroupID, prefix) == "" || len(selection.VMSSModelDigest) != 64 {
		return nil, nil, e2bbe.ErrUnsupported
	}
	selection.ParentHostID = "primary"
	if !selection.Disabled {
		selection.ProfileDigest = ""
		selection.ProfileDigest = elasticHash([]any{selection, elasticHash(base)})
	} else if len(selection.ProfileDigest) != 64 {
		return nil, nil, e2bbe.ErrUnsupported
	}
	arm, err := newAzureARMReader(base.AzureSecrets.ClientID)
	if err != nil {
		return nil, nil, err
	}
	var pool *api.CloudTaskPool
	guard := func(c context.Context, r e2bbe.Request, b int64) error {
		if pool == nil {
			return management.ErrExecutionAuthority
		}
		return pool.AuthorizeAllocation(c, "primary", r, b)
	}
	runtime, closeRuntime, err := openE2BTaskRuntimeSingle(ctx, cfg, projects, authorize, guard)
	if err != nil {
		return nil, nil, err
	}
	fail := func(err error) (*api.CloudTaskRuntime, func(), error) { closeRuntime(); return nil, nil, err }
	root, err := os.OpenRoot(base.ControllerStateDir)
	if err != nil {
		return fail(err)
	}
	err = root.Mkdir("host-pool", 0700)
	if err != nil && !errors.Is(err, os.ErrExist) {
		root.Close()
		return fail(err)
	}
	info, err := root.Lstat("host-pool")
	root.Close()
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return fail(e2bbe.ErrUnsupported)
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return fail(err)
	}
	host := api.CloudTaskHost{ID: "primary", Name: "Configured Azure host", ConfigDigest: elasticHash(json.RawMessage(raw)), HostKey: elasticHash([]string{base.HostNodeID}), Runtime: runtime}
	pool, err = api.NewCloudTaskPool(api.CloudTaskPoolConfig{StateDir: filepath.Join(base.ControllerStateDir, "host-pool"), Scope: base.Deployment.Scope, DefaultHostID: "primary", Limits: base.Limits}, []api.CloudTaskHost{host})
	if err != nil {
		return fail(err)
	}
	driver := &azureElasticDriver{selection: selection, base: base, worker: cfg, projects: projects, authorize: authorize, arm: arm}
	if !selection.Disabled {
		if err = rememberAzureElasticSelection(base, selection); err != nil {
			pool.Close()
			return fail(err)
		}
	}
	if err = pool.ConfigureAzureElasticity(selection.AzureElasticOperator, driver, selection.PendingTaskSlots); err != nil {
		pool.Close()
		return fail(err)
	}
	result := *runtime
	result.Pool = pool
	return &result, func() { pool.Close(); closeRuntime() }, nil
}

type rememberedAzureElastic struct {
	Version    int                   `json:"version"`
	OwnerID    string                `json:"ownerId"`
	Scope      string                `json:"scope"`
	DataDiskID string                `json:"dataDiskId"`
	Selection  azureElasticSelection `json:"selection"`
}

func readRememberedAzureElastic(base e2bOperatorConfig) (*azureElasticSelection, error) {
	if base.Deployment.Provider != "azure" || !filepath.IsAbs(base.ControllerStateDir) {
		return nil, nil
	}
	return readRememberedAzureElasticAt(base, filepath.Join(base.ControllerStateDir, "host-pool"))
}
func readRememberedAzureElasticAt(base e2bOperatorConfig, dir string) (*azureElasticSelection, error) {
	path := filepath.Join(dir, "elastic-operator.json")
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := readPrivateE2BFile(path, 96<<10)
	if err != nil {
		return nil, err
	}
	var stored rememberedAzureElastic
	if json.Unmarshal(raw, &stored) != nil || stored.Version != 1 || stored.OwnerID != base.Store.OwnerID || stored.Scope != base.Store.RuntimeScope || stored.DataDiskID != base.Store.AzureDataDiskID {
		return nil, e2bbe.ErrConflict
	}
	stored.Selection.Disabled = true
	return &stored.Selection, nil
}
func rememberAzureElasticSelection(base e2bOperatorConfig, selection azureElasticSelection) error {
	return rememberAzureElasticSelectionAt(base, selection, filepath.Join(base.ControllerStateDir, "host-pool"))
}
func rememberAzureElasticSelectionAt(base e2bOperatorConfig, selection azureElasticSelection, dir string) error {
	raw, err := json.Marshal(rememberedAzureElastic{1, base.Store.OwnerID, base.Store.RuntimeScope, base.Store.AzureDataDiskID, selection})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".elastic-operator-")
	if err != nil {
		return err
	}
	name := f.Name()
	// This is replaceable CURRENT metadata; original host requests/configs and
	// immutable resource identities live separately and are never overwritten.
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(raw)
	}
	if err == nil {
		err = f.Sync()
	}
	closed := f.Close()
	if err != nil || closed != nil {
		return errors.Join(err, closed)
	}
	if err = os.Rename(name, filepath.Join(dir, "elastic-operator.json")); err != nil {
		return err
	}
	root, err := os.Open(dir)
	if err != nil {
		return err
	}
	return errors.Join(root.Sync(), root.Close())
}
