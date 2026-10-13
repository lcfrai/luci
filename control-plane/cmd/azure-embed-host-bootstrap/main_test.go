package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeSecrets map[string][]byte

func (s fakeSecrets) Get(_ context.Context, ref secretRef) ([]byte, error) {
	if value := s[ref.Name]; len(value) != 0 {
		return value, nil
	}
	return nil, errors.New("secret absent")
}

func hostFixture(t *testing.T) (config, fakeSecrets) {
	t.Helper()
	root := t.TempDir()
	private := filepath.Join(root, "private")
	if err := os.Mkdir(private, 0700); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(root, "base.env")
	compose := filepath.Join(root, "compose.yaml")
	composeBytes := []byte("services:\n  api:\n    image: pinned-example\n")
	if err := os.WriteFile(compose, composeBytes, 0600); err != nil {
		t.Fatal(err)
	}
	data := []byte("E2B_API_IMAGE=example@sha256:known\nE2B_CLIENT_PROXY_IMAGE=example@sha256:known\n")
	if err := os.WriteFile(base, data, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	composeSum := sha256.Sum256(composeBytes)
	c := config{VaultURL: "https://luci-private.vault.azure.net", GatewayHost: "embed.private.example",
		PrivateListenIP: "10.20.0.4", ControllerIP: "10.20.0.5", Scope: "scope-one", Generation: "generation-one",
		HostClientID: "12345678-1234-1234-1234-123456789abc", ControllerClientID: "87654321-1234-1234-1234-123456789abc",
		PrivateDirectory: private, ComposePath: compose, ComposeSHA256: hex.EncodeToString(composeSum[:]),
		ComposeEnvBase: base, ComposeEnvSHA256: hex.EncodeToString(sum[:]),
		TeamAPIKey: secretRef{"team-key", strings.Repeat("a", 32)}, AdminToken: secretRef{"admin-token", strings.Repeat("b", 32)},
		SandboxTokenSeed:  secretRef{"sandbox-seed", strings.Repeat("c", 32)},
		GatewayServerCert: secretRef{"server-cert", strings.Repeat("d", 32)}, GatewayServerKey: secretRef{"server-key", strings.Repeat("e", 32)},
		GatewayClientCA: secretRef{"client-ca", strings.Repeat("f", 32)}}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	certificate := func(ca bool, names []string) []byte {
		t.Helper()
		template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "private-host"}, DNSNames: names,
			NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: ca, BasicConstraintsValid: true,
			KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
		der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	}
	privateKey, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return c, fakeSecrets{
		"team-key": []byte("e2b_" + strings.Repeat("1", 32)), "admin-token": []byte(strings.Repeat("2", 64)),
		"sandbox-seed": []byte(strings.Repeat("3", 64)),
		"server-cert":  certificate(false, []string{c.GatewayHost, "*." + c.GatewayHost}),
		"server-key":   pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKey}),
		"client-ca":    certificate(true, nil),
	}
}

func TestAzureHostStagesOnlyPinnedScopedPrivateMaterials(t *testing.T) {
	c, values := hostFixture(t)
	if !validConfig(c) {
		t.Fatal("separate host identity and exact refs refused")
	}
	result, err := stage(context.Background(), c, values, func(string) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(result.Directory) })
	for _, path := range []string{result.ComposeEnv, result.ServerCert, result.ServerKey, result.ClientCA, result.GatewayConfig} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || filepath.Dir(path) != result.Directory {
			t.Fatal("host output was not a private owned file", path, err)
		}
	}
	env, err := os.ReadFile(result.ComposeEnv)
	if err != nil || !strings.Contains(string(env), "TEAM_API_KEY=e2b_") || !strings.Contains(string(env), "ADMIN_TOKEN=") ||
		!strings.Contains(string(env), "SANDBOX_ACCESS_TOKEN_HASH_SEED=") {
		t.Fatal("pinned runtime environment omitted selected exact secrets", err)
	}
	gateway, err := os.ReadFile(result.GatewayConfig)
	if err != nil || !strings.Contains(string(gateway), `"controllerPrivateIp":"10.20.0.5"`) ||
		!strings.Contains(string(gateway), `"generation":"generation-one"`) || strings.Contains(string(gateway), "e2b_111") {
		t.Fatal("host did not bind a nonsecret exact gateway input", err)
	}
	unknown := filepath.Join(result.Directory, "unrelated")
	if err := os.WriteFile(unknown, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := retire(c, result.Directory, func(string) bool { return true }); err == nil {
		t.Fatal("retirement removed an unknown file")
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}
	if err := retire(c, result.Directory, func(string) bool { return true }); err != nil {
		t.Fatal("exact staged secrets did not retire", err)
	}
	if _, err := os.Lstat(result.Directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("staged secret directory survived retirement", err)
	}
	wrong := c
	wrong.ControllerClientID = c.HostClientID
	if validConfig(wrong) {
		t.Fatal("controller identity reused for host secrets")
	}
	wrong = c
	wrong.ComposeSHA256 = strings.Repeat("0", 64)
	if _, err := stage(context.Background(), wrong, values, func(string) bool { return true }); err == nil {
		t.Fatal("changed Compose source accepted")
	}
	wrong = c
	wrong.ComposeEnvSHA256 = strings.Repeat("0", 64)
	if _, err := stage(context.Background(), wrong, values, func(string) bool { return true }); err == nil {
		t.Fatal("changed base runtime environment accepted")
	}
	unsafeBase := []byte("export TEAM_API_KEY = another-key\n")
	if err := os.WriteFile(c.ComposeEnvBase, unsafeBase, 0600); err != nil {
		t.Fatal(err)
	}
	unsafeSum := sha256.Sum256(unsafeBase)
	wrong = c
	wrong.ComposeEnvSHA256 = hex.EncodeToString(unsafeSum[:])
	if _, err := stage(context.Background(), wrong, values, func(string) bool { return true }); err == nil {
		t.Fatal("base runtime environment overrode managed secret")
	}
	if _, err := stage(context.Background(), c, values, func(string) bool { return false }); err == nil {
		t.Fatal("non-tmpfs secret output accepted")
	}
}

func TestAzureHostRefusesMissingSecretsAndCleansPartialOutput(t *testing.T) {
	c, values := hostFixture(t)
	delete(values, "client-ca")
	if _, err := stage(context.Background(), c, values, func(string) bool { return true }); err == nil || strings.Contains(err.Error(), "secret absent") {
		t.Fatal("missing host secret did not fail privately", err)
	}
	entries, err := os.ReadDir(c.PrivateDirectory)
	if err != nil || len(entries) != 0 {
		t.Fatal("failed host materialization left files", err, entries)
	}
	values["client-ca"] = []byte("not a CA")
	if _, err := stage(context.Background(), c, values, func(string) bool { return true }); err == nil {
		t.Fatal("invalid gateway client CA was accepted")
	}
	entries, err = os.ReadDir(c.PrivateDirectory)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid certificate left host secret files", err, entries)
	}
}
