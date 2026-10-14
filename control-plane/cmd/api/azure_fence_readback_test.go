package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fixedAzureFence struct {
	record azureFenceRecord
	err    error
}

func (f fixedAzureFence) Read(context.Context, string) (azureFenceRecord, error) {
	return f.record, f.err
}

func TestAzureControllerFenceStartupAndEffectBoundary(t *testing.T) {
	selected, _ := azureARMFixture()
	oldID := selected.AzureMount.ControllerInstanceID
	newID := strings.Replace(oldID, "controller-one", "controller-two", 1)
	selected.Store.OwnerID = "owner-one"
	selected.Store.RuntimeScope = "scope-one"
	selected.AzureMount.FilesystemUUID = "12345678-1234-1234-1234-123456789abc"
	selected.AzureMount.FenceBlobURL = "https://lucifence.blob.core.windows.net/controller-fence/controller-fence.json"
	selected.AzureMount.Generation = "generation-two"
	selected.AzureMount.ControllerInstanceID = newID
	priorFence := strings.Repeat("f", 64)
	generation := azureGenerationReceipt{Version: 1, Generation: "generation-two", ControllerInstanceID: newID,
		AzureDataDiskID: selected.Store.AzureDataDiskID, PreviousGeneration: "generation-one", PreviousFenceReceiptDigest: priorFence}
	data, err := json.Marshal(generation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "generation.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	selected.AzureMount.GenerationReceiptPath = path
	selected.AzureMount.GenerationReceiptDigest = hex.EncodeToString(hash[:])
	record := azureFenceRecord{Version: 1, State: "api_start_pending", DiskID: selected.Store.AzureDataDiskID,
		FilesystemUUID: selected.AzureMount.FilesystemUUID, OwnerID: selected.Store.OwnerID, Scope: selected.Store.RuntimeScope,
		ControllerID: oldID, Generation: "generation-one", TargetControllerID: newID, TargetGeneration: "generation-two",
		GenerationReceiptSHA256: selected.AzureMount.GenerationReceiptDigest, FenceReceiptDigest: priorFence}
	ctx := context.Background()
	if err := verifyAzureFence(ctx, selected, fixedAzureFence{record: record}, true); err != nil {
		t.Fatal("exact replacement startup was refused", err)
	}
	if err := verifyAzureFence(ctx, selected, fixedAzureFence{record: record}, false); err == nil {
		t.Fatal("restoration-pending controller was allowed to dispatch")
	}
	record.State = "active"
	record.ControllerID, record.Generation = newID, "generation-two"
	record.TargetControllerID, record.TargetGeneration = "", ""
	if err := verifyAzureFence(ctx, selected, fixedAzureFence{record: record}, false); err != nil {
		t.Fatal("settled active generation was refused", err)
	}
	for _, altered := range []func(*azureFenceRecord){
		func(r *azureFenceRecord) { r.ControllerID = oldID },
		func(r *azureFenceRecord) { r.GenerationReceiptSHA256 = strings.Repeat("a", 64) },
		func(r *azureFenceRecord) { r.FenceReceiptDigest = strings.Repeat("b", 64) },
		func(r *azureFenceRecord) { r.State = "detach_pending" },
		func(r *azureFenceRecord) { r.OwnerID = "another-owner" },
	} {
		bad := record
		altered(&bad)
		if err := verifyAzureFence(ctx, selected, fixedAzureFence{record: bad}, false); err == nil {
			t.Fatal("wrong or uncertain off-disk writer was admitted", bad)
		}
	}
	if err := verifyAzureFence(ctx, selected, fixedAzureFence{err: errors.New("unavailable")}, false); err == nil {
		t.Fatal("unreadable external fence was accepted")
	}
}
