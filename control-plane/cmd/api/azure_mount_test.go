//go:build linux

package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAzureGatewayClientCertificateBindsControllerGeneration(t *testing.T) {
	uri, _ := url.Parse(azureControllerSAN("scope-one", "generation-one"))
	leaf := &x509.Certificate{URIs: []*url.URL{uri}}
	if !azureControllerCertificateMatches(leaf, "scope-one", "generation-one") ||
		azureControllerCertificateMatches(leaf, "scope-one", "generation-two") ||
		azureControllerCertificateMatches(leaf, "other-scope", "generation-one") {
		t.Fatal("Azure mTLS identity was not bound to exact scope and generation")
	}
}

func TestAzureDurableMountBindsEveryControllerPathAndReceipt(t *testing.T) {
	mount := t.TempDir()
	makeDir := func(name string) string {
		path := filepath.Join(mount, name)
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	uuid := "12345678-1234-1234-1234-123456789abc"
	selected := e2bOperatorConfig{}
	selected.Store.OwnerID = "owner-one"
	selected.Store.RuntimeScope = "scope-one"
	selected.Store.ControllerHostID = "controller-host"
	selected.Store.RuntimeHostID = "runtime-host"
	selected.Store.AzureDataDiskID = "/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/luci/providers/microsoft.compute/disks/luci-data"
	selected.Deployment.AzureDataDiskID = selected.Store.AzureDataDiskID
	selected.Store.Directory = makeDir("artifacts")
	selected.ControllerStateDir = makeDir("controller")
	selected.StagingDir = makeDir("staging")
	selected.TemplateCreateJournalPath = filepath.Join(makeDir("templates"), "create.json")
	selected.TemplateBuildJournalPath = filepath.Join(makeDir("templates"), "build.json")
	cfg := Config{StateDir: makeDir("state"), ProjectsDir: makeDir("projects"), ExecutionStateDir: makeDir("execution")}
	receipt := azureDiskReceipt{Version: 1, AzureDataDiskID: selected.Store.AzureDataDiskID, FilesystemUUID: uuid,
		OwnerID: selected.Store.OwnerID, Scope: selected.Store.RuntimeScope,
		ControllerHostID: selected.Store.ControllerHostID, RuntimeHostID: selected.Store.RuntimeHostID}
	data, _ := json.Marshal(receipt)
	path := filepath.Join(mount, "disk-receipt.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	selected.Store.SeparationReceiptDigest = hex.EncodeToString(hash[:])
	generation := azureGenerationReceipt{Version: 1, Generation: "controller-generation-one",
		ControllerInstanceID: "/subscriptions/12345678-1234-1234-1234-123456789abc/resourcegroups/luci/providers/microsoft.compute/virtualmachines/controller-one",
		AzureDataDiskID:      selected.Store.AzureDataDiskID}
	generationData, _ := json.Marshal(generation)
	generationPath := filepath.Join(mount, "generation.json")
	if err := os.WriteFile(generationPath, generationData, 0600); err != nil {
		t.Fatal(err)
	}
	generationHash := sha256.Sum256(generationData)
	m := azureDurableMount{MountPath: mount, FilesystemUUID: uuid, ReceiptPath: path,
		Generation: generation.Generation, ControllerInstanceID: generation.ControllerInstanceID,
		GenerationReceiptPath: generationPath, GenerationReceiptDigest: hex.EncodeToString(generationHash[:])}
	probe := func(string, string) (uint64, error) { return azurePathDevice(mount), nil }
	if err := verifyAzureDurableMountWithProbe(m, selected, cfg, probe); err != nil {
		t.Fatal(err)
	}
	other := m
	other.Generation = "controller-generation-two"
	if err := verifyAzureDurableMountWithProbe(other, selected, cfg, probe); err == nil {
		t.Fatal("old controller generation was accepted")
	}
	generation.PreviousGeneration = "controller-generation-zero"
	generation.PreviousFenceReceiptDigest = strings.Repeat("b", 64)
	generationData, _ = json.Marshal(generation)
	if err := os.WriteFile(generationPath, generationData, 0600); err != nil {
		t.Fatal(err)
	}
	generationHash = sha256.Sum256(generationData)
	m.GenerationReceiptDigest = hex.EncodeToString(generationHash[:])
	if err := verifyAzureDurableMountWithProbe(m, selected, cfg, probe); err != nil {
		t.Fatal("fenced replacement generation was refused", err)
	}
	bad := selected
	bad.StagingDir = t.TempDir()
	if err := verifyAzureDurableMountWithProbe(m, bad, cfg, probe); err == nil {
		t.Fatal("off-disk task transfer was accepted")
	}
	bad = selected
	bad.Store.AzureDataDiskID += "-other"
	if err := verifyAzureDurableMountWithProbe(m, bad, cfg, probe); err == nil {
		t.Fatal("wrong ARM disk identity was accepted")
	}
	if err := os.WriteFile(path, append(data, ' '), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyAzureDurableMountWithProbe(m, selected, cfg, probe); err == nil {
		t.Fatal("changed on-disk identity receipt was accepted")
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(mount, "moved")); err != nil {
		t.Fatal(err)
	}
	if azurePathOnMount(mount, filepath.Join(mount, "moved", "state"), azurePathDevice(mount)) {
		t.Fatal("linked path escaped durable controller disk")
	}
}

func TestAzureMountInfoRequiresExactWritableExt4(t *testing.T) {
	row := []byte("36 25 8:16 / /luci/data rw,relatime - ext4 /dev/sdb rw\n")
	device, ok := azureMountInfoDevice(row, "/luci/data")
	if !ok || device == 0 {
		t.Fatal("exact ext4 mount refused")
	}
	for _, changed := range []string{
		strings.Replace(string(row), " /luci/data ", " /luci/data-old ", 1),
		strings.Replace(string(row), " rw,relatime ", " ro,relatime ", 1),
		strings.Replace(string(row), " - ext4 ", " - tmpfs ", 1),
	} {
		if _, accepted := azureMountInfoDevice([]byte(changed), "/luci/data"); accepted {
			t.Fatal("wrong disk mount was accepted", changed)
		}
	}
}

func TestAzureGuestOriginsStayAtSelectedGateway(t *testing.T) {
	if !validAzureGuestOriginTemplate("https://49983-{sandboxID}.embed.private.example", "49983", "embed.private.example") ||
		!validAzureGuestOriginTemplate("https://8080-{sandboxID}.embed.private.example", "8080", "embed.private.example") {
		t.Fatal("pinned gateway origins were refused")
	}
	for _, value := range []string{
		"https://49983-{sandboxID}.other.private.example",
		"https://{sandboxID}.embed.private.example",
		"https://49983-{sandboxID}.embed.private.example:5008",
	} {
		if validAzureGuestOriginTemplate(value, "49983", "embed.private.example") {
			t.Fatal("unpinned guest destination was accepted", value)
		}
	}
}
