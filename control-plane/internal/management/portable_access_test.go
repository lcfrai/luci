package management

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPortableOwnerAccessIsPrivatePerRunAndNotPublicOrInherited(t *testing.T) {
	root := t.TempDir()
	opts := Options{StateDir: root, InitialHandler: http.NotFoundHandler(), PortableOwnerAccess: true}
	g, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close(context.Background())
	path := filepath.Join(root, "control-plane", "owner-access-token")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	capability := strings.TrimSpace(string(data))
	if len(capability) != 43 || capability == g.bootstrapToken || capability != g.opts.LocalAccessToken {
		t.Fatal("portable capability is missing or reuses the bootstrap proof")
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0600) {
		t.Fatalf("portable capability is not private: %v", err)
	}
	h := g.Handler()
	localRequest(t, h, "GET", "/v1/auth/local-owner", capability, "127.0.0.1:1", nil, 200)
	localRequest(t, h, "GET", "/v1/auth/local-owner", "", "127.0.0.1:1", nil, 403)
	localRequest(t, h, "GET", "/v1/auth/local-owner", capability, "192.0.2.1:1", nil, 403)
	request(t, h, "POST", "/v1/auth/bootstrap", "", map[string]string{"bootstrapToken": g.bootstrapToken, "username": "owner", "displayName": "Owner", "password": testPassword}, 201)
	localRequest(t, h, "POST", "/v1/auth/local-owner/recover", capability, "127.0.0.1:1", map[string]string{"newPassword": "another portable owner password"}, 200)
	public := request(t, h, "GET", "/v1/control-plane/status", "", nil, 200).Body.Bytes()
	identity, err := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{capability, "owner-access-token", "PortableOwnerAccess", "localAccess"} {
		if bytes.Contains(public, []byte(forbidden)) || bytes.Contains(identity, []byte(forbidden)) {
			t.Fatal("portable proof exposed through public status or persistent identity")
		}
	}
	env := isolatedEnvironment([]string{"LUCI_PORTABLE_OWNER_ACCESS=1", "LUCI_LOCAL_ACCESS_TOKEN=" + capability}, t.TempDir(), "worker-key", "address", User{ID: "member", Role: "member"})
	if strings.Contains(strings.Join(env, "\n"), "LUCI_PORTABLE_OWNER_ACCESS") || strings.Contains(strings.Join(env, "\n"), capability) {
		t.Fatal("portable authority inherited by a worker")
	}
	if err := g.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("owned proof file remained after close: %v", err)
	}
	next, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close(context.Background())
	if next.opts.LocalAccessToken == capability {
		t.Fatal("portable capability reused across launches")
	}
	localRequest(t, next.Handler(), "GET", "/v1/auth/local-owner", capability, "127.0.0.1:1", nil, 403)
	// A late cleanup from the old process must preserve the replacement proof.
	if err := g.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	replacement, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(replacement)) != next.opts.LocalAccessToken {
		t.Fatal("old gateway removed another launch's proof")
	}
}

func TestPortableOwnerAccessRefusesMixedProofAndNativeDoesNotWriteProof(t *testing.T) {
	root := t.TempDir()
	opts := Options{StateDir: root, InitialHandler: http.NotFoundHandler(), LocalAccessToken: localTestCapability, PortableOwnerAccess: true}
	if _, err := New(opts); err == nil {
		t.Fatal("mixed portable and native capability accepted")
	}
	opts.PortableOwnerAccess = false
	g, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close(context.Background())
	if _, err := os.Stat(filepath.Join(root, "control-plane", "owner-access-token")); !os.IsNotExist(err) {
		t.Fatalf("native launch wrote a portable proof: %v", err)
	}
}
