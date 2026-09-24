package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The sandbox git fix's three variables: PANACEA_APP_GIT_NAME/EMAIL as a pair,
// and PANACEA_APP_CA_BUNDLE, validated at startup.

// Each CA refusal is its own errCABundle* value, so each test asks errors.Is
// whether it got its own and not another's. Never the error's text: on Linux,
// where CI runs, the generic "cannot be read: %w" fallback carries the OS's
// errno string, "no such file or directory" or "is a directory", and a text
// match could not tell the fallback from the refusal it replaced.
//
// caSentence is the one place the product's own words are matched: run()
// prints the error, so its output is text. These are words validateCABundle
// writes and no operating system does.
const caSentence = "Export your network's root certificate as PEM"

// caRefusal fails the test unless err is refusal want and none of the others.
func caRefusal(t *testing.T, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Errorf("error %v, want the refusal %q", err, want)
	}
	if !strings.Contains(fmt.Sprint(err), envCABundle) {
		t.Errorf("error %v does not name %s", err, envCABundle)
	}
	for _, other := range []error{errCABundleNotAbsolute, errCABundleMissing, errCABundleIsADirectory, errCABundleNoCertificate} {
		if other != want && errors.Is(err, other) {
			t.Errorf("error %v is also the refusal %q", err, other)
		}
	}
}

// caWrite writes content to a fresh file and returns its absolute path.
func caWrite(t *testing.T, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.pem")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// caCertPEM is a real self-signed certificate, generated rather than pasted so
// the repository carries no certificate at all.
func caCertPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "panacea-app test root"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestGitIdentityIsAPairOrNothing(t *testing.T) {
	cfg, err := loadConfig(env(map[string]string{envGitName: " Ada Lovelace ", envGitEmail: "ada@example.invalid"}))
	if err != nil {
		t.Fatalf("both set: %v", err)
	}
	if cfg.GitName != "Ada Lovelace" || cfg.GitEmail != "ada@example.invalid" {
		t.Errorf("GitName/GitEmail = %q/%q", cfg.GitName, cfg.GitEmail)
	}

	cfg, err = loadConfig(env(nil))
	if err != nil || cfg.GitName != "" || cfg.GitEmail != "" {
		t.Errorf("neither set: %q/%q, %v; want both empty and no error", cfg.GitName, cfg.GitEmail, err)
	}

	for set, missing := range map[string]string{envGitName: envGitEmail, envGitEmail: envGitName} {
		_, err := loadConfig(env(map[string]string{set: "x"}))
		if err == nil {
			t.Errorf("only %s set: accepted", set)
			continue
		}
		if !strings.HasPrefix(err.Error(), set+" is set but "+missing+" is not") {
			t.Errorf("only %s set: error %q, want it to name %s as set and %s as missing", set, err, set, missing)
		}
	}
}

// Unset is today's behaviour exactly: no bundle, and nothing is read.
func TestNoCABundleIsTodaysBehaviour(t *testing.T) {
	cfg, err := loadConfig(env(nil))
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.CABundle != "" {
		t.Errorf("CABundle = %q with the variable unset", cfg.CABundle)
	}
}

func TestAValidCABundleIsAccepted(t *testing.T) {
	path := caWrite(t, caCertPEM(t))
	cfg, err := loadConfig(env(map[string]string{envCABundle: path}))
	if err != nil {
		t.Fatalf("a real PEM certificate was refused: %v", err)
	}
	if cfg.CABundle != path {
		t.Errorf("CABundle = %q, want %q", cfg.CABundle, path)
	}
	// A bundle is several certificates, and one of them being a key or junk
	// does not make the others untrustworthy.
	mixed := append(append([]byte("# exported 2026-09-23\n"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1}})...), caCertPEM(t)...)
	if _, err := loadConfig(env(map[string]string{envCABundle: caWrite(t, mixed)})); err != nil {
		t.Errorf("a bundle with one certificate among other blocks was refused: %v", err)
	}
}

// Missing: an absolute path that would otherwise be fine. The missing file is
// the only thing wrong, so it is the condition that decides.
func TestAMissingCABundleStopsStartupWithItsOwnSentence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-exported.pem")
	_, err := loadConfig(env(map[string]string{envCABundle: path}))
	if err == nil {
		t.Fatal("a missing bundle was accepted")
	}
	caRefusal(t, err, errCABundleMissing)
}

// No certificate: the file exists and is readable, and each case is valid PEM
// or plausibly so — so "holds a certificate" is the only thing that decides.
func TestACABundleWithNoCertificateStopsStartupWithItsOwnSentence(t *testing.T) {
	for name, content := range map[string][]byte{
		"a PEM block that is not a certificate": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte{1, 2, 3}}),
		"a CERTIFICATE block x509 cannot parse": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not DER")}),
		// The label decides, not only the bytes: Node reads NODE_EXTRA_CA_CERTS
		// with OpenSSL's PEM_read_bio_X509, which skips a block not labelled as
		// a certificate, so a real certificate under another label trusts
		// nothing. Every other case here also fails x509's parse, so without
		// this one, dropping the label check would pass every test.
		"a real certificate under a label that is not CERTIFICATE": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: caDER(t)}),
		"a binary DER export, not PEM":                             caDER(t),
		"an empty file":                                            {},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadConfig(env(map[string]string{envCABundle: caWrite(t, content)}))
			if err == nil {
				t.Fatal("accepted")
			}
			caRefusal(t, err, errCABundleNoCertificate)
		})
	}
}

func caDER(t *testing.T) []byte {
	t.Helper()
	block, _ := pem.Decode(caCertPEM(t))
	return block.Bytes
}

// The path must be absolute and must be a file. Each refusal names itself.
func TestACABundlePathMustBeAnAbsoluteFile(t *testing.T) {
	for value, want := range map[string]error{
		"corp-root.pem": errCABundleNotAbsolute,
		t.TempDir():     errCABundleIsADirectory,
	} {
		_, err := loadConfig(env(map[string]string{envCABundle: value}))
		if err == nil {
			t.Errorf("%s=%q was accepted", envCABundle, value)
			continue
		}
		caRefusal(t, err, want)
	}
}

// "Stop startup" is the process, not the parser: run exits 2 with the
// sentence, before anything listens.
//
// HERMETIC ON PURPOSE, and the reason was measured. With only the bundle
// set, a regression that skips the check turns this test into a real
// control plane: the default state directory, the default 127.0.0.1:8730
// and the real Docker daemon, whose reaper sweeps at startup and destroys
// every managed container it does not own. A mutation campaign did exactly
// that. (Since the reaper-scope fix it destroys only orphans of the default
// state directory's own instance — which is the real user's, so the hazard
// is narrower, not gone.) So the backend is kata with a kubeconfig set, which kubeClient
// refuses outright, even inside a cluster — serve() then fails in newBackend,
// before the reaper, a registry or a listener exists — and the state
// directory and address are this test's own. The check intact still exits 2;
// the check broken exits 1, fast, and touches nothing.
func TestRunRefusesAMissingCABundleWithExitCode2(t *testing.T) {
	var out bytes.Buffer
	path := filepath.Join(t.TempDir(), "never-exported.pem")
	if code := run(env(runHermetic(t, map[string]string{envCABundle: path})), &out); code != 2 {
		t.Fatalf("exit code = %d, want 2; output %q", code, out.String())
	}
	if !strings.Contains(out.String(), caSentence) {
		t.Errorf("output %q does not carry the sentence %q", out.String(), caSentence)
	}
}

// runHermetic is the environment every test that calls run() uses: the
// variables under test over a control plane that cannot reach anything if the
// check under test is broken. kata with a kubeconfig, which kubeClient refuses
// outright even inside a cluster, so serve() fails in newBackend before the
// reaper, a registry or a listener exists; and this test's own state
// directory and port 0, so even a serve() that got further would touch
// neither the default state directory nor 127.0.0.1:8730. The one thing
// serve() does write before newBackend is the instance id it mints, and it
// writes it inside that state directory (instance/instance-id). Every key the test
// sets wins, except the backend and kubeconfig, which a test may not change.
func runHermetic(t *testing.T, vars map[string]string) map[string]string {
	t.Helper()
	m := map[string]string{
		envStateDir: t.TempDir(),
		envAddr:     "127.0.0.1:0",
	}
	for k, v := range vars {
		m[k] = v
	}
	m[envBackend] = "kata"
	m[envKubeconfig] = "kubeClient-refuses-any-kubeconfig"
	return m
}
