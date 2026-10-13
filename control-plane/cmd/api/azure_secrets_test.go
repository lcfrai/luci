package main

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
)

type fakeAzureSecretSource struct {
	fail string
	seen []string
}

func (f *fakeAzureSecretSource) GetSecret(_ context.Context, name, version string) ([]byte, error) {
	f.seen = append(f.seen, name+"@"+version)
	if name == f.fail {
		return nil, errors.New("fixture failure")
	}
	return []byte("fixture-" + name), nil
}

func azureSecretsFixture(t *testing.T) azureSecretMaterialization {
	t.Helper()
	ref := func(name string) azureSecretRef { return azureSecretRef{Name: name, Version: strings.Repeat("a", 32)} }
	return azureSecretMaterialization{VaultURL: "https://luci.vault.azure.net", ClientID: "12345678-1234-1234-1234-123456789abc",
		Directory: t.TempDir(), APIKey: ref("task-key"), AdminKey: ref("admin-key"), AttachKey: ref("attach-key"), ClientCert: ref("client-cert"), ClientKey: ref("client-key")}
}

func TestAzureSecretsMaterializePrivateExactVersionsAndCleanUp(t *testing.T) {
	cfg := azureSecretsFixture(t)
	source := &fakeAzureSecretSource{}
	files, cleanup, err := materializeAzureSecrets(context.Background(), cfg, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{files.APIKey, files.AdminKey, files.AttachKey, files.ClientCert, files.ClientKey} {
		data, readErr := readPrivateE2BFile(path, 4096)
		if readErr != nil || !strings.HasPrefix(string(data), "fixture-") {
			t.Fatal("materialized secret was missing or public", readErr)
		}
	}
	if len(source.seen) != 5 || source.seen[0] != "task-key@"+strings.Repeat("a", 32) {
		t.Fatal("selected secret versions changed", source.seen)
	}
	cleanup()
	if _, err = os.Lstat(files.APIKey); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned secret was retained after close", err)
	}
}

func TestAzureSecretsRefuseIncompleteReadWithoutRetainingFiles(t *testing.T) {
	cfg := azureSecretsFixture(t)
	source := &fakeAzureSecretSource{fail: "client-key"}
	if _, _, err := materializeAzureSecrets(context.Background(), cfg, source); err == nil {
		t.Fatal("missing client key was accepted")
	}
	entries, err := os.ReadDir(cfg.Directory)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed materialization left credentials", err, entries)
	}
	cfg.VaultURL = "https://other.example"
	if _, _, err = materializeAzureSecrets(context.Background(), cfg, source); err == nil {
		t.Fatal("non-Key-Vault source was accepted")
	}
	cfg = azureSecretsFixture(t)
	cfg.Directory = "/dev/shm" // shared tmpfs parent, not owner-only
	if _, _, err = materializeAzureSecrets(context.Background(), cfg, source); err == nil {
		t.Fatal("shared secret directory was accepted")
	}
}
