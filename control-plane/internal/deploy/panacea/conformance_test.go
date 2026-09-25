package panacea_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea/panaceatest"
)

type tokens string

func (s tokens) PanaceaToken(context.Context) (string, error) { return string(s), nil }

// unbuilt stands in for what *panacea.Client does not implement yet: deploy.Actor and
// deploy.Governor (task 04) and Reader.Stream (task 12). Each method SKIPS the conformance row
// that reached it, naming the task, so the row's reads before that call still run over the
// connector and no row reads as passed that did not run. It goes when Client implements
// deploy.Deployer: embedding both then gives each method an ambiguous selector, so connector
// stops satisfying deploy.Deployer and this file stops compiling until unbuilt is deleted.
type unbuilt struct{ t *testing.T }

func (u unbuilt) skip(method, task string) {
	u.t.Helper()
	u.t.Skipf("%s over the connector is task %s's; this row's reads before it ran", method, task)
}

func (u unbuilt) Stream(context.Context, deploy.ProjectRef, deploy.StreamKind, string) (deploy.Stream, error) {
	u.skip("Stream", "12")
	return nil, nil
}
func (u unbuilt) Ship(context.Context, deploy.ProjectRef, deploy.ShipRequest) (deploy.ShipResult, error) {
	u.skip("Ship", "04")
	return deploy.ShipResult{}, nil
}
func (u unbuilt) Cancel(context.Context, deploy.ProjectRef, string) (deploy.CancelResult, error) {
	u.skip("Cancel", "04")
	return deploy.CancelResult{}, nil
}
func (u unbuilt) Rollback(context.Context, deploy.ProjectRef, string) (deploy.RollbackResult, error) {
	u.skip("Rollback", "04")
	return deploy.RollbackResult{}, nil
}
func (u unbuilt) AutoDeploy(context.Context, deploy.ProjectRef, string) (deploy.AutoDeploy, error) {
	u.skip("AutoDeploy", "04")
	return deploy.AutoDeploy{}, nil
}
func (u unbuilt) SetAutoDeploy(context.Context, deploy.ProjectRef, string, bool) (deploy.AutoDeploy, error) {
	u.skip("SetAutoDeploy", "04")
	return deploy.AutoDeploy{}, nil
}
func (u unbuilt) Posture(context.Context, deploy.ProjectRef, string) (deploy.Posture, error) {
	u.skip("Posture", "04")
	return deploy.Posture{}, nil
}
func (u unbuilt) SetPosture(context.Context, deploy.ProjectRef, string, deploy.PostureChange) (deploy.PostureResult, error) {
	u.skip("SetPosture", "04")
	return deploy.PostureResult{}, nil
}
func (u unbuilt) Access(context.Context, deploy.ProjectRef) ([]deploy.AccessGrant, error) {
	u.skip("Access", "04")
	return nil, nil
}
func (u unbuilt) Grant(context.Context, deploy.ProjectRef, deploy.GrantRequest) error {
	u.skip("Grant", "04")
	return nil
}
func (u unbuilt) Revoke(context.Context, deploy.ProjectRef, string) error {
	u.skip("Revoke", "04")
	return nil
}
func (u unbuilt) SetURLPrefix(context.Context, deploy.ProjectRef, string, string) (deploy.URLPrefix, error) {
	u.skip("SetURLPrefix", "04")
	return deploy.URLPrefix{}, nil
}
func (u unbuilt) ClearURLPrefix(context.Context, deploy.ProjectRef, string) error {
	u.skip("ClearURLPrefix", "04")
	return nil
}

// connector is the real *panacea.Client (Identity and every Reader method but Stream) plus
// unbuilt for the rest.
type connector struct {
	*panacea.Client
	unbuilt
}

var _ deploy.Deployer = connector{}

func panaceaHarness(t *testing.T) (deploy.Deployer, deploytest.Seeder) {
	srv := panaceatest.New(t)
	c, err := panacea.New(panacea.Config{
		Origin:     srv.URL(),
		Tokens:     tokens(srv.Token()),
		HTTPClient: srv.HTTPClient(),
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("panacea.New against panaceatest: %v", err)
	}
	return connector{c, unbuilt{t}}, srv
}

// TestConformanceOverConnector kills the connector and the Fake disagreeing about any row:
// the same suite, against panaceatest's Panacea envelopes through the real transport.
func TestConformanceOverConnector(t *testing.T) {
	deploytest.Conformance(t, panaceaHarness)
}
