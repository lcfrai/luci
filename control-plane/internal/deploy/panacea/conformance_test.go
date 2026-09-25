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

// unbuilt stands in for what *panacea.Client does not implement yet: Reader.Stream (task 12).
// Task 04 deleted its Actor and Governor stand-ins when Client gained those methods. It SKIPS the
// conformance row that reached it, naming the task, so the row's reads before that call still
// run over the connector and no row reads as passed that did not run. It goes when Client
// implements deploy.Deployer: embedding both then gives Stream an ambiguous selector, so
// connector stops satisfying deploy.Deployer and this file stops compiling until unbuilt is
// deleted.
type unbuilt struct{ t *testing.T }

func (u unbuilt) skip(method, task string) {
	u.t.Helper()
	u.t.Skipf("%s over the connector is task %s's; this row's reads before it ran", method, task)
}

func (u unbuilt) Stream(context.Context, deploy.ProjectRef, deploy.StreamKind, string) (deploy.Stream, error) {
	u.skip("Stream", "12")
	return nil, nil
}

// connector is the real *panacea.Client (Identity, Actor, Governor and every Reader method but
// Stream) plus unbuilt for Stream.
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
