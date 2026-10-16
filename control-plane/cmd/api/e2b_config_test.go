package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func TestAzureGatewayTrustsOnlySelectedPrivateRoot(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-time.Hour),
		NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	selected := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	pool, err := cloudProviderRoots(selected, true)
	if err != nil || len(pool.Subjects()) != 1 {
		t.Fatal("Azure gateway inherited unrelated trust roots", err)
	}
	if _, err := cloudProviderRoots([]byte("not a certificate"), true); err == nil {
		t.Fatal("Azure gateway accepted invalid private root")
	}
}

type cloudStartupFixture struct {
	rows                     []e2bbe.Record
	templateErr, nodeErr     error
	templateReads, nodeReads int
}

func (f *cloudStartupFixture) Snapshot() ([]e2bbe.Record, e2bbe.Usage) {
	return f.rows, e2bbe.Usage{}
}
func (f *cloudStartupFixture) VerifyTemplateBuild(context.Context) error {
	f.templateReads++
	return f.templateErr
}
func (f *cloudStartupFixture) VerifyNodeCapacity(context.Context) error {
	f.nodeReads++
	return f.nodeErr
}

func TestCloudStartupKeepsRetainedOwnershipVisibleDuringProviderOutage(t *testing.T) {
	unavailable := errors.New("provider temporarily unreachable")
	for _, row := range []e2bbe.Record{{State: e2bbe.Creating, ResourcesHeld: true}, {State: e2bbe.Allocated, ResourcesHeld: true}, {State: e2bbe.Unknown, ResourcesHeld: true}, {State: e2bbe.Cancelling, ResourcesHeld: true}, {State: e2bbe.Absent}} {
		f := &cloudStartupFixture{rows: []e2bbe.Record{row}, templateErr: unavailable, nodeErr: unavailable}
		if err := verifyCloudStartup(context.Background(), f); err != nil || f.templateReads != 1 || f.nodeReads != 1 {
			t.Fatalf("retained %s was hidden by unavailable provider: %v, reads %d/%d", row.State, err, f.templateReads, f.nodeReads)
		}
	}
	for _, rows := range [][]e2bbe.Record{nil, {{State: e2bbe.Absent, ProductOutcome: "handoff"}}} {
		f := &cloudStartupFixture{rows: rows, templateErr: unavailable}
		if err := verifyCloudStartup(context.Background(), f); !errors.Is(err, unavailable) || f.templateReads != 1 || f.nodeReads != 0 {
			t.Fatalf("new cloud allocation was offered without template proof: %v, reads %d/%d", err, f.templateReads, f.nodeReads)
		}
		f = &cloudStartupFixture{rows: rows, nodeErr: unavailable}
		if err := verifyCloudStartup(context.Background(), f); !errors.Is(err, unavailable) || f.templateReads != 1 || f.nodeReads != 1 {
			t.Fatalf("new cloud allocation was offered without host proof: %v, reads %d/%d", err, f.templateReads, f.nodeReads)
		}
	}
	f := &cloudStartupFixture{nodeErr: e2bbe.ErrCapacity}
	if err := verifyCloudStartup(context.Background(), f); err != nil {
		t.Fatal("an observed full node should backpressure task admission, not hide the API", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f = &cloudStartupFixture{rows: []e2bbe.Record{{State: e2bbe.Unknown, ResourcesHeld: true}}}
	if err := verifyCloudStartup(ctx, f); !errors.Is(err, context.Canceled) || f.templateReads != 0 {
		t.Fatal("cancelled startup continued to provider probes", err)
	}
}

func TestCloudOperatorConfigRejectsPublicLinkedAndUntrustedOrigins(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "operator.json")
	if err := os.WriteFile(private, []byte(`{"scope":"owned"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readPrivateE2BFile(private, 1024); err != nil || string(data) != `{"scope":"owned"}` {
		t.Fatal("private operator config was unreadable", err)
	}
	if _, err := readPrivateE2BFile(private, 4); err == nil {
		t.Fatal("oversized operator config was accepted")
	}
	linked := filepath.Join(root, "linked.json")
	if err := os.Symlink(private, linked); err == nil {
		if _, err = readPrivateE2BFile(linked, 1024); err == nil {
			t.Fatal("linked operator config was accepted")
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(private, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := readPrivateE2BFile(private, 1024); err == nil {
			t.Fatal("public operator config was accepted")
		}
	}
	good := "https://{sandboxID}.guests.example.test"
	if !validE2BOriginTemplate(good) {
		t.Fatal("private authenticated guest origin was refused")
	}
	for _, value := range []string{"http://{sandboxID}.guests.example.test", "https://{sandboxID}.guests.example.test:5008", "https://{sandboxID}.guests.example.test/x", "https://user@{sandboxID}.guests.example.test", "https://{sandboxID}.guests.example.test#fragment", "https://{sandboxID}.guests.example.test?x=1", strings.ReplaceAll(good, "{sandboxID}", "fixed")} {
		if validE2BOriginTemplate(value) {
			t.Fatalf("untrusted guest origin was accepted: %s", value)
		}
	}
}

func TestCloudRevocationRetainsOnlyExactOwnedObservationAndCleanupEffects(t *testing.T) {
	for _, action := range []string{"cancel", "observe", "guest-transport", "observe-launch", "observe-native-exit", "capture-output", "review-output"} {
		if !cloudOwnershipOnlyEffect(action) {
			t.Fatalf("prior owned attempt cannot be settled after revocation: %s", action)
		}
	}
	for _, action := range []string{"allocate", "stage-input", "start", "import-source", "", "deploy"} {
		if cloudOwnershipOnlyEffect(action) {
			t.Fatalf("new or unrecognized effect inherited cleanup authority: %s", action)
		}
	}
}

type cloudContractEffectFixture struct {
	readErr, budgetErr            error
	reads, validations            int
	projectID, runID, operationID string
}

func (f *cloudContractEffectFixture) GetProjectSwarmRun(projectID, runID string) (product.ProjectRun, error) {
	f.reads++
	f.projectID, f.runID = projectID, runID
	return product.ProjectRun{ID: runID, ProjectID: projectID}, f.readErr
}
func (f *cloudContractEffectFixture) ValidateSwarmOperationExecutionContract(projectID string, run product.ProjectRun, operationID string) error {
	f.validations++
	f.operationID = operationID
	if projectID != f.projectID || run.ID != f.runID || run.ProjectID != projectID {
		return management.ErrExecutionAuthority
	}
	return f.budgetErr
}

func TestCloudControllerActualEffectCallbackRechecksRepairBudgetAndPreservesCleanup(t *testing.T) {
	request := e2bbe.Request{Identity: e2bbe.Identity{ProjectID: "project-a", RunID: "run-a", AttemptID: "operation-a", OperationID: "child-a"}}
	reader := &cloudContractEffectFixture{}
	checks := 0
	callback := cloudOperationEffectAuthorizer(reader, func(context.Context, e2bbe.Request, string) error {
		checks++
		return nil // fake current exact-owner/provider/grant/spend authority
	})
	if err := callback(context.Background(), request, "allocate"); err != nil || checks != 1 || reader.reads != 1 || reader.validations != 1 || reader.operationID != request.Identity.AttemptID {
		t.Fatal("allocation did not bind the exact current product operation", err, checks, reader)
	}
	// A policy changes during transfer after the VM already exists. Every
	// subsequent controller effect consumes the same current product gate.
	reader.budgetErr = errors.New("current approved budget no longer permits repair")
	for _, action := range []string{"stage-input", "start", "allocate"} {
		if err := callback(context.Background(), request, action); !errors.Is(err, management.ErrExecutionAuthority) {
			t.Fatal("changed budget reached a new controller effect", action, err)
		}
	}
	beforeReads, beforeValidations := reader.reads, reader.validations
	for _, action := range []string{"cancel", "observe", "guest-transport", "observe-launch", "observe-native-exit", "capture-output", "review-output"} {
		if err := callback(context.Background(), request, action); err != nil || reader.reads != beforeReads || reader.validations != beforeValidations {
			t.Fatal("revoked new-work budget blocked exact owned cleanup", action, err)
		}
	}
	denied := cloudOperationEffectAuthorizer(reader, func(context.Context, e2bbe.Request, string) error { return management.ErrExecutionAuthority })
	if err := denied(context.Background(), request, "cancel"); !errors.Is(err, management.ErrExecutionAuthority) || reader.reads != beforeReads {
		t.Fatal("cleanup budget exception bypassed current owner authority", err)
	}
	reader.budgetErr, reader.readErr = nil, errors.New("current product record unavailable")
	if err := callback(context.Background(), request, "start"); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("missing product record permitted start", err)
	}
	if err := callback(context.Background(), request, "unrecognized-effect"); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("unknown effect inherited new-work authority", err)
	}
}
