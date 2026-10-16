package backend

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFPilotTwoRolesHaveOneOnceOnlyIdentityEach(t *testing.T) {
	scope, attempt := strings.Repeat("a", 64), strings.Repeat("b", 32)
	renderer := PDFPilotAllocationID(scope, attempt, PDFPilotRenderer)
	reader := PDFPilotAllocationID(scope, attempt, PDFPilotReader)
	if len(renderer) != 64 || len(reader) != 64 || renderer == reader || renderer != PDFPilotAllocationID(scope, attempt, PDFPilotRenderer) {
		t.Fatal("exact serial child identities were not stable and distinct")
	}
	if PDFPilotAllocationID(scope, attempt, "web") != "" || PDFPilotAllocationID("bad", attempt, PDFPilotReader) != "" {
		t.Fatal("unscoped or unsupported role gained an allocation identity")
	}
	common := PDFPilotIdentity{
		Protocol: PDFPilotProtocol, ScopeDigest: scope, AttemptID: attempt, ActorDigest: strings.Repeat("c", 64),
		ApprovalDigest: strings.Repeat("8", 64),
		EngineID:       "selected-engine", InstanceID: "selected-instance", SourceCommit: strings.Repeat("d", 40),
		ImageID: "sha256:" + strings.Repeat("e", 64), ImageReceiptDigest: strings.Repeat("f", 64),
		BinaryReceiptDigest: strings.Repeat("1", 64), PackageReceiptDigest: strings.Repeat("2", 64),
		OriginalFontDigest: strings.Repeat("3", 64), OriginalFontReceipt: strings.Repeat("4", 64),
		OutputStageDigest: strings.Repeat("5", 64), CommandDigest: strings.Repeat("6", 64),
	}
	root := t.TempDir()
	render := common
	render.Role, render.AllocationID = PDFPilotRenderer, renderer
	if !ValidatePDFPilotIdentity(render) || !ValidatePDFPilotSpec(PDFPilotSpec{
		Identity: render, OutputStagePath: filepath.Join(root, attempt, "renderer"),
		OriginalFontPath: filepath.Join(root, "font.ttc"), OriginalFontBytes: PDFPilotFontBytes, UserID: 10001, GroupID: 10001,
	}) {
		t.Fatal("trusted-shape fixed renderer identity refused")
	}
	if ValidatePDFPilotSpec(PDFPilotSpec{
		Identity: render, OutputStagePath: filepath.Join(root, attempt, "renderer"),
		OriginalFontPath: filepath.Join(root, "font.ttc"), OriginalFontBytes: PDFPilotFontBytes - 1, UserID: 10001, GroupID: 10001,
	}) {
		t.Fatal("changed original font byte count was admitted")
	}
	read := common
	read.Role, read.AllocationID, read.RendererOutputDigest = PDFPilotReader, reader, strings.Repeat("7", 64)
	if !ValidatePDFPilotIdentity(read) || !ValidatePDFPilotSpec(PDFPilotSpec{
		Identity: read, OutputStagePath: filepath.Join(root, attempt, "reader"),
		RendererPDFPath:  filepath.Join(root, attempt, "renderer", "pilot.pdf"),
		RendererPDFBytes: 1024, UserID: 10001, GroupID: 10001,
	}) {
		t.Fatal("bound reader identity refused")
	}
	read.RendererOutputDigest = ""
	if ValidatePDFPilotIdentity(read) {
		t.Fatal("reader without the durable renderer output digest was admitted")
	}
}
