package e2bbe

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

func transferBinding() TransferBinding {
	r := taskRequest("alpha", "one")
	return TransferBinding{Identity: r.Identity, SourceDigest: r.SourceDigest, ContextDigest: r.ContextDigest, Direction: "input"}
}
func payload(name, body string) Payload {
	return Payload{TransferFile: TransferFile{Path: name, Size: int64(len(body)), Mode: 0644, SHA256: bytesDigest([]byte(body))}, Open: func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(body)), nil }}
}
func TestTransferRoundTripPreservesApprovedContextAndUpstreamInstructions(t *testing.T) {
	b := transferBinding()
	files := []Payload{payload("source/CLAUDE.md", "upstream data"), payload("context/approved.json", `{"revision":17}`), payload("source/AGENTS.md", "existing instructions"), payload("source/main.go", "package main")}
	var a, again bytes.Buffer
	m, hash, e := WriteTransfer(&a, b, files)
	if e != nil {
		t.Fatal(e)
	}
	if _, _, e = WriteTransfer(&again, b, files); e != nil || !bytes.Equal(a.Bytes(), again.Bytes()) {
		t.Fatal("noncanonical archive", e)
	}
	got, e := VerifyTransfer(bytes.NewReader(a.Bytes()), b, hash)
	if e != nil || digest(got) != digest(m) {
		t.Fatal(got, e)
	}
	if a.Len() > 8192 {
		t.Fatal("unexpected archive size")
	}
}
func TestTransferRejectsCrossProjectStaleContextAlteredArchiveAndDirection(t *testing.T) {
	b := transferBinding()
	var a bytes.Buffer
	_, hash, e := WriteTransfer(&a, b, []Payload{payload("source/main.go", "package main")})
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"project", "attempt", "context", "source", "direction", "digest", "content", "trailing"} {
		t.Run(name, func(t *testing.T) {
			want := b
			h := hash
			data := append([]byte(nil), a.Bytes()...)
			switch name {
			case "project":
				want.Identity.ProjectID = "beta"
			case "attempt":
				want.Identity.AttemptID = "another"
			case "context":
				want.ContextDigest = strings.Repeat("4", 64)
			case "source":
				want.SourceDigest = strings.Repeat("5", 64)
			case "direction":
				want.Direction = "output"
			case "digest":
				h = strings.Repeat("6", 64)
			case "content":
				at := bytes.Index(data, []byte("package main"))
				data[at] = 'X'
			case "trailing":
				data = append(data, []byte("extra")...)
				h = bytesDigest(data)
			}
			if _, e := VerifyTransfer(bytes.NewReader(data), want, h); !errors.Is(e, ErrTransfer) {
				t.Fatal("unsafe transfer accepted", e)
			}
		})
	}
}
func TestTransferRejectsUnsafePathsWithoutOpeningPayload(t *testing.T) {
	names := []string{"../escape", "/source/file", "source/../escape", "source/.git/config", "source/dir\\file", "source/colon:stream", "source/CON", "source/a.", "source/a ", "source/NUL.txt", "source/LPT1.txt", "source/a?", "source/a*", "source/a|", "source/nonascii-é", "output/result", "source//file"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			v := payload(name, "data")
			v.Open = func() (io.ReadCloser, error) { t.Error("unsafe payload opened"); return nil, nil }
			if _, _, e := WriteTransfer(io.Discard, transferBinding(), []Payload{v}); !errors.Is(e, ErrTransfer) {
				t.Fatal(e)
			}
		})
	}
}
func TestTransferRejectsCaseCollisionFileDirectoryConflictAndLimits(t *testing.T) {
	cases := map[string][]Payload{"case": {payload("source/A", "one"), payload("source/a", "two")}, "parent": {payload("source/a", "one"), payload("source/a/b", "two")}, "parent-case": {payload("source/A/b", "one"), payload("source/a", "two")}, "oversize": {payload("source/a", "one")}}
	cases["oversize"][0].Size = MaxTransferBytes + 1
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			if _, _, e := WriteTransfer(io.Discard, transferBinding(), files); !errors.Is(e, ErrTransfer) {
				t.Fatal(e)
			}
		})
	}
	files := make([]Payload, MaxTransferFiles+1)
	if _, _, e := WriteTransfer(io.Discard, transferBinding(), files); !errors.Is(e, ErrTransfer) {
		t.Fatal(e)
	}
}
func TestTransferRejectsStreamChangeAndNeverExtracts(t *testing.T) {
	for _, name := range []string{"short", "long", "hash", "nil"} {
		t.Run(name, func(t *testing.T) {
			v := payload("source/a", "expected")
			switch name {
			case "short":
				v.Size++
			case "long":
				v.Size--
			case "hash":
				v.SHA256 = strings.Repeat("7", 64)
			case "nil":
				v.Open = func() (io.ReadCloser, error) { return nil, nil }
			}
			if _, _, e := WriteTransfer(io.Discard, transferBinding(), []Payload{v}); !errors.Is(e, ErrTransfer) {
				t.Fatal(e)
			}
		})
	}
}
func TestReturnedArtifactsAreDataAndRejectLinksPAXAndUnlistedFiles(t *testing.T) {
	b := transferBinding()
	b.Direction = "output"
	v := payload("output/result.patch", "untrusted model claims")
	var normal bytes.Buffer
	m, hash, e := WriteTransfer(&normal, b, []Payload{v})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = VerifyTransfer(bytes.NewReader(normal.Bytes()), b, hash); e != nil {
		t.Fatal(e)
	}
	for _, variant := range []string{"symlink", "hardlink", "pax", "unlisted", "manifest-field"} {
		t.Run(variant, func(t *testing.T) {
			var out bytes.Buffer
			tw := tar.NewWriter(&out)
			body, _ := json.Marshal(m)
			if variant == "manifest-field" {
				body = append(body[:len(body)-1], []byte(`,"execute":true}`)...)
			}
			if e := tw.WriteHeader(&tar.Header{Name: "manifest.json", Typeflag: tar.TypeReg, Size: int64(len(body)), Mode: 0600, Format: tar.FormatUSTAR}); e != nil {
				t.Fatal(e)
			}
			tw.Write(body)
			h := &tar.Header{Name: v.Path, Typeflag: tar.TypeReg, Size: v.Size, Mode: 0600, Format: tar.FormatUSTAR}
			switch variant {
			case "symlink":
				h.Typeflag = tar.TypeSymlink
				h.Linkname = "../../outside"
				h.Size = 0
			case "hardlink":
				h.Typeflag = tar.TypeLink
				h.Linkname = "output/other"
				h.Size = 0
			case "pax":
				h.Format = tar.FormatPAX
				h.PAXRecords = map[string]string{"comment": "ignored?"}
			}
			if e := tw.WriteHeader(h); e != nil {
				t.Fatal(e)
			}
			if h.Size > 0 {
				tw.Write([]byte("untrusted model claims"))
			}
			if variant == "unlisted" {
				tw.WriteHeader(&tar.Header{Name: "output/extra", Typeflag: tar.TypeReg, Size: 0, Mode: 0600, Format: tar.FormatUSTAR})
			}
			if e := tw.Close(); e != nil {
				t.Fatal(e)
			}
			if _, e := VerifyTransfer(bytes.NewReader(out.Bytes()), b, bytesDigest(out.Bytes())); !errors.Is(e, ErrTransfer) {
				t.Fatal("unsafe return accepted", e)
			}
		})
	}
}

func TestTransferRetainsExecutableSourceModeAndRejectsPrivilegeModes(t *testing.T) {
	b := transferBinding()
	v := payload("source/build.sh", "#!/bin/sh\nexit 0\n")
	v.Mode = 0755
	var out bytes.Buffer
	m, hash, e := WriteTransfer(&out, b, []Payload{v})
	if e != nil {
		t.Fatal(e)
	}
	got, e := VerifyTransfer(bytes.NewReader(out.Bytes()), b, hash)
	if e != nil || got.Files[0].Mode != 0755 || m.Files[0].Mode != 0755 {
		t.Fatal(got, e)
	}
	for _, mode := range []uint32{0, 04755, 06755, 0777} {
		v.Mode = mode
		if _, _, e := WriteTransfer(io.Discard, b, []Payload{v}); !errors.Is(e, ErrTransfer) {
			t.Fatal("privileged/unrepresentable mode accepted", mode, e)
		}
	}
	v = payload("context/approved.json", "{}")
	v.Mode = 0755
	if _, _, e := WriteTransfer(io.Discard, b, []Payload{v}); !errors.Is(e, ErrTransfer) {
		t.Fatal("context executable", e)
	}
}
