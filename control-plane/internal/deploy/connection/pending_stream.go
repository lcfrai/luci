package connection

import (
	"context"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/panacea"
)

// PanaceaDeployer is *panacea.Client as a deploy.Deployer.
//
// IT IS TEMPORARY AND THE COMPILER DELETES IT. *panacea.Client implements every method of
// deploy.Deployer except Reader.Stream, which is task 12's, so a closure over panacea.New
// cannot be a NewClient today - which task 05's plan assumed it could. Rather than widen
// Config.NewClient to *panacea.Client (which would shut deploytest.Fake and every other
// double out of it), this is the same device panacea's conformance test already uses for the
// same gap: embed the real client, supply the one missing method, and let the embedding
// become an AMBIGUOUS SELECTOR the moment Client gains Stream. At that point this file stops
// compiling and task 12 deletes it, replacing the one call site with the bare client.
//
// Its Stream is not a stub that pretends: it is KindRejected with a sentence naming what has
// not been built, so nothing reads a nil stream as an empty one.
type PanaceaDeployer struct {
	*panacea.Client
	pendingStream
}

var _ deploy.Deployer = PanaceaDeployer{}

type pendingStream struct{}

func (pendingStream) Stream(context.Context, deploy.ProjectRef, deploy.StreamKind, string) (deploy.Stream, error) {
	return nil, &deploy.Error{
		Kind:    deploy.KindRejected,
		Op:      "stream",
		Message: "Live logs are not built yet in this cockpit.",
	}
}
