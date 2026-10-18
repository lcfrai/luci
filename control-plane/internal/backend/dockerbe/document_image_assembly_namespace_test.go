package dockerbe

import (
	"context"
	"testing"
)

func TestAssemblyNamespaceEmptyAcquisitionsAndTokensRefuse(t *testing.T) {
	ctx := context.Background()
	for _, a := range []*DocumentImageAssemblyNamespaceAcquisition{nil, {}} {
		if _, err := a.RegistryLease(); err == nil { t.Fatal("empty acquisition issued lease") }
		if g, err := a.Acquire(ctx); err == nil || g != nil { t.Fatal("empty acquisition issued token") }
		a.RequestRetirement()
	}
	for _, token := range []*assemblyNamespaceToken{nil, {}} {
		if token.CheckCurrent(ctx) == nil || token.Close() == nil { t.Fatal("empty token accepted") }
		if g, err := token.Transfer(ctx); err == nil || g != nil { t.Fatal("empty token moved authority") }
	}
}

func TestAssemblyNamespaceCopiedShellRetirementCannotMintCustody(t *testing.T) {
	// Missing original guard is deliberately not a directory acquisition.
	o := &assemblyNamespaceOwner{generation: 1, ready: true}
	a := &DocumentImageAssemblyNamespaceAcquisition{state: &assemblyOriginalAcquisition{namespace: o}}
	copyShell := *a
	copyShell.RequestRetirement()
	if !a.state.retired.Load() || !o.closing || o.ready { t.Fatal("shell copy lost shared retirement") }
	if g, err := a.Acquire(context.Background()); err == nil || g != nil { t.Fatal("missing original owner became acquired") }
	if _, err := a.RegistryLease(); err == nil { t.Fatal("missing original registry manufactured charge") }
	token := &assemblyNamespaceToken{o, 1}
	if token.CheckCurrent(context.Background()) == nil || token.Close() == nil { t.Fatal("private metadata alone became custody") }
}
