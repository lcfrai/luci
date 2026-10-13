package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func returnedSourceFixture(t *testing.T) (Request, TransferManifest, TransferManifest, CaptureReceipt, []byte, []byte) {
	t.Helper()
	req := taskRequest("alpha", "one")
	inputBinding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	input := TransferManifest{Version: 1, Binding: inputBinding, Files: []TransferFile{
		payload("context/approved.json", "approved").TransferFile,
		payload("source/AGENTS.md", "instructions").TransferFile,
		payload("source/main.go", "old").TransferFile,
	}}
	outputBinding := inputBinding
	outputBinding.Direction = "output"
	source := SourceSnapshot{Version: 1, Policy: SourceSnapshotPolicy, InputBundleSHA256: req.BundleDigest, Files: []TransferFile{
		payload("AGENTS.md", "instructions").TransferFile,
		payload("main.go", "new").TransferFile,
		payload("new.go", "added").TransferFile,
	}}
	original := input.Files[2]
	original.Path = "main.go"
	updated := source.Files[1]
	added := source.Files[2]
	sourceBytes, _ := json.Marshal(source)
	changes := SourceChanges{Version: 1, Policy: SourceSnapshotPolicy, InputBundleSHA256: req.BundleDigest, SourceManifestSHA256: bytesDigest(sourceBytes), Changes: []SourceChange{
		{Path: "main.go", Kind: "modified", Before: &original, After: &updated},
		{Path: "new.go", Kind: "added", After: &added},
	}, Exclusions: []SourceExclusion{{Path: "node_modules", Reason: "fixed-new-path-exclusion"}}}
	changeBytes, _ := json.Marshal(changes)
	output := TransferManifest{Version: 1, Binding: outputBinding, Files: []TransferFile{
		payload("output/changes.json", string(changeBytes)).TransferFile,
		payload("output/source-manifest.json", string(sourceBytes)).TransferFile,
		payload("output/source/AGENTS.md", "instructions").TransferFile,
		payload("output/source/main.go", "new").TransferFile,
		payload("output/source/new.go", "added").TransferFile,
	}}
	receipt := CaptureReceipt{ArchiveSHA256: strings.Repeat("a", 64), SourceManifestSHA256: bytesDigest(sourceBytes), ChangeManifestSHA256: bytesDigest(changeBytes)}
	return req, input, output, receipt, sourceBytes, changeBytes
}

func TestStoredReturnedSourceRequiresCompleteBoundInputAndOutput(t *testing.T) {
	req := taskRequest("alpha", "one")
	inBinding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	var inputArchive bytes.Buffer
	_, inputHash, err := WriteTransfer(&inputArchive, inBinding, []Payload{payload("context/approved.json", "approved"), payload("source/main.go", "old")})
	if err != nil {
		t.Fatal(err)
	}
	req.BundleDigest = inputHash
	old := payload("main.go", "old").TransferFile
	newFile := payload("main.go", "new").TransferFile
	source := SourceSnapshot{Version: 1, Policy: SourceSnapshotPolicy, InputBundleSHA256: inputHash, Files: []TransferFile{newFile}}
	sourceBytes, _ := json.Marshal(source)
	change := SourceChanges{Version: 1, Policy: SourceSnapshotPolicy, InputBundleSHA256: inputHash, SourceManifestSHA256: bytesDigest(sourceBytes), Changes: []SourceChange{{Path: "main.go", Kind: "modified", Before: &old, After: &newFile}}}
	changeBytes, _ := json.Marshal(change)
	outBinding := inBinding
	outBinding.Direction = "output"
	var outputArchive bytes.Buffer
	_, outputHash, err := WriteTransfer(&outputArchive, outBinding, []Payload{payload("output/changes.json", string(changeBytes)), payload("output/source-manifest.json", string(sourceBytes)), payload("output/source/main.go", "new")})
	if err != nil {
		t.Fatal(err)
	}
	receipt := CaptureReceipt{ArchiveSHA256: outputHash, ArchiveBytes: int64(outputArchive.Len()), SourceManifestSHA256: bytesDigest(sourceBytes), ChangeManifestSHA256: bytesDigest(changeBytes)}
	store := openStore(t, storeConfig(t))
	if _, err = store.PutTransfer(context.Background(), inBinding, inputHash, bytes.NewReader(inputArchive.Bytes())); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ValidateStoredReturnedSource(context.Background(), req, receipt); err == nil {
		t.Fatal("missing output archive accepted")
	}
	if _, err = store.PutTransfer(context.Background(), outBinding, outputHash, bytes.NewReader(outputArchive.Bytes())); err != nil {
		t.Fatal(err)
	}
	got, err := store.ValidateStoredReturnedSource(context.Background(), req, receipt)
	if err != nil || len(got.Changes) != 1 || got.Changes[0].Path != "main.go" {
		t.Fatal("stored source was not independently verified", got, err)
	}
	wrong := receipt
	wrong.ArchiveBytes++
	if _, err = store.ValidateStoredReturnedSource(context.Background(), req, wrong); !errors.Is(err, ErrTransfer) {
		t.Fatal("wrong archive length accepted", err)
	}
}

func TestReturnedSourceBindsExactInputOutputAndDiffBeforeReview(t *testing.T) {
	req, input, output, receipt, sourceBytes, changeBytes := returnedSourceFixture(t)
	got, err := ValidateReturnedSource(req, input, output, receipt, sourceBytes, changeBytes)
	if err != nil || len(got.Changes) != 2 {
		t.Fatal("exact returned source rejected", got, err)
	}
	checks := []struct {
		name   string
		change func(*Request, *TransferManifest, *TransferManifest, *CaptureReceipt, *[]byte, *[]byte)
	}{
		{"wrong-project", func(r *Request, _ *TransferManifest, _ *TransferManifest, _ *CaptureReceipt, _ *[]byte, _ *[]byte) {
			r.Identity.ProjectID = "other"
		}},
		{"wrong-input-bundle", func(r *Request, _ *TransferManifest, _ *TransferManifest, _ *CaptureReceipt, _ *[]byte, _ *[]byte) {
			r.BundleDigest = strings.Repeat("b", 64)
		}},
		{"missing-source-file", func(_ *Request, _ *TransferManifest, out *TransferManifest, _ *CaptureReceipt, _ *[]byte, _ *[]byte) {
			out.Files = out.Files[:len(out.Files)-1]
		}},
		{"false-source-hash", func(_ *Request, _ *TransferManifest, out *TransferManifest, _ *CaptureReceipt, _ *[]byte, _ *[]byte) {
			out.Files[3].SHA256 = strings.Repeat("c", 64)
		}},
		{"unlisted-change", func(_ *Request, _ *TransferManifest, _ *TransferManifest, rc *CaptureReceipt, _ *[]byte, cb *[]byte) {
			var c SourceChanges
			_ = json.Unmarshal(*cb, &c)
			c.Changes = c.Changes[:1]
			*cb, _ = json.Marshal(c)
			rc.ChangeManifestSHA256 = bytesDigest(*cb)
		}},
		{"hide-original", func(_ *Request, _ *TransferManifest, _ *TransferManifest, rc *CaptureReceipt, _ *[]byte, cb *[]byte) {
			var c SourceChanges
			_ = json.Unmarshal(*cb, &c)
			c.Exclusions = []SourceExclusion{{Path: "AGENTS.md", Reason: "fixed-new-path-exclusion"}}
			*cb, _ = json.Marshal(c)
			rc.ChangeManifestSHA256 = bytesDigest(*cb)
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			r, in, out, rec := req, input, output, receipt
			sb, cb := append([]byte(nil), sourceBytes...), append([]byte(nil), changeBytes...)
			in.Files = append([]TransferFile(nil), input.Files...)
			out.Files = append([]TransferFile(nil), output.Files...)
			check.change(&r, &in, &out, &rec, &sb, &cb)
			if _, err := ValidateReturnedSource(r, in, out, rec, sb, cb); !errors.Is(err, ErrTransfer) {
				t.Fatal("unbound task source accepted", err)
			}
		})
	}
}
