package api

import (
	"context"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestDebianBinaryTransportSelectionCopiesFiniteMetadataWithoutAuthority(t *testing.T) {
	request := backend.DocumentImageAssemblyBinaryReadRequest{Purpose: backend.DocumentImageAssemblyBinaryReadPurpose, LoanID: "one"}
	request.Packages[173].Role = "retained"
	selection := copyDebianBinaryTransportSelection(request)
	request.Packages[173].Role = "changed"; request.LoanID = "changed"
	if selection.request.Packages[173].Role != "retained" || selection.request.LoanID != "one" { t.Fatal("selection aliases mutable request arrays") }
	// Invalid metadata remains metadata: the backend Reserve, not a new syntax
	// validator/Verified flag, validates the complete174/four/three request.
	if selection.request.ScopeDigest != "" { t.Fatal("metadata copy fabricated source authority") }
}

func TestDebianBinaryTransportMissingAdmissionCannotReserveOrConsume(t *testing.T) {
	owner := newDebianProofNativeOwner(context.Background())
	got, err := owner.prepareBinaryReadTransport(context.Background(), debianBinaryTransportSelection{}, nil, nil, nil)
	if err == nil || got != nil { t.Fatal("missing registered original owners became reservation authority") }
	if owner.census != nil && len(owner.census.transports) != 0 { t.Fatal("refused input installed a transport") }
}

func TestDebianBinaryTransportUnsupportedPlatformCannotPrepare(t *testing.T) {
	if debianProducerPlatformSupported() { t.Skip("unsupported-platform boundary only") }
	owner := newDebianProofNativeOwner(context.Background())
	got, err := owner.prepareBinaryReadTransport(context.Background(), debianBinaryTransportSelection{}, &debianProofPackageAdmission{}, &debianProofEvidenceAdmission{}, &debianProofEvidenceAdmission{})
	if err == nil || got != nil || owner.duplicateBinaryOriginalAlias(context.Background(), nil, 0) == nil || owner.settleBinaryReadTransport(context.Background(), nil) == nil { t.Fatal("unsupported platform fabricated original aliases or settlement") }
}
