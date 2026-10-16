package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/png"
	"io"
	"sort"
	"strings"
	"testing"
)

func TestPDFPilotDriverRejectsArbitraryInputAndParserRoutes(t *testing.T) {
	base := []string{"--role", roleRead, "--attempt", strings.Repeat("a", 32), "--allocation", strings.Repeat("b", 64), "--input-sha256", strings.Repeat("c", 64)}
	if _, err := parseRequest(base); err != nil { t.Fatal(err) }
	for _, change := range []func([]string) []string{
		func(v []string) []string { return append(v, "--tool", "/bin/sh") },
		func(v []string) []string { v[1] = "run"; return v },
		func(v []string) []string { v[3] = "../outside"; return v },
		func(v []string) []string { v[5] = strings.Repeat("B", 64); return v },
		func(v []string) []string { v[6] = "--url"; return v },
		func(v []string) []string { v[1] = roleRender; return v },
	} {
		v := append([]string(nil), base...)
		if _, err := parseRequest(change(v)); err == nil { t.Fatal("arbitrary route or wrong original font accepted") }
	}
}

func TestPDFPilotDriverPDFInfoProvesOnePageBeforeOtherReaders(t *testing.T) {
	for _, fixture := range []struct { text string; accepted bool }{
		{"Title: pilot\nPages:           1\nPage size: 595 x 842 pts\n", true},
		{"Pages: 2\n", false},
		{"Pages: 1\nPages: 2\n", false},
		{"Pages: 1\nPages: 1\n", false},
		{"Page size: 595 x 842 pts\n", false},
		{"Pages: one\n", false},
		{"Pages: 1\x00\n", false},
		{"Pages: 1\n" + strings.Repeat("x", maxPDFInfo), false},
	} {
		if onePDFInfoPage([]byte(fixture.text)) != fixture.accepted { t.Fatalf("page-count parsing differed for %q", fixture.text) }
	}
}

type inertPilotStage struct { files map[string][]byte; closed bool }
func (s *inertPilotStage) Read(name string, maximum int) ([]byte, error) {
	data, ok := s.files[name]
	if !ok || len(data) > maximum { return nil, errRefused }
	return append([]byte(nil), data...), nil
}
func (s *inertPilotStage) Only(expected ...string) error {
	names := make([]string, 0, len(s.files))
	for name := range s.files { names = append(names, name) }
	sort.Strings(names)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	if len(names) != len(want) { return errRefused }
	for i := range names { if names[i] != want[i] { return errRefused } }
	return nil
}
func (s *inertPilotStage) Close() error { s.closed = true; return nil }

func pngPageFixture(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := png.Encode(&out, image.NewRGBA(image.Rect(0, 0, 595, 842))); err != nil { t.Fatal(err) }
	return out.Bytes()
}

func TestPDFPilotDriverRunsFixedInfoTextPageSequenceAndFramesOnlyResults(t *testing.T) {
	pdf := []byte("fixed PDF source fixture")
	digest := sha256.Sum256(pdf)
	r := frameFixture(roleRead)
	r.inputDigest = hex.EncodeToString(digest[:])
	stage := &inertPilotStage{files: map[string][]byte{"pilot.txt": []byte("日本語\n"), "page.png": pngPageFixture(t)}}
	var paths []string
	ops := driverOps{
		readInput: func(path string, limit int) ([]byte, error) {
			if path != pdfPath || limit != maxPDF { return nil, errRefused }
			return pdf, nil
		},
		newStage: func(attempt string) (pilotStage, error) {
			if attempt != r.attempt { return nil, errRefused }
			return stage, nil
		},
		runTool: func(_ context.Context, path string, args []string, cap int) ([]byte, error) {
			paths = append(paths, path+" "+strings.Join(args, " "))
			if path == pdfinfoPath && cap == maxPDFInfo { return []byte("Pages: 1\n"), nil }
			if cap != 0 { return nil, errRefused }
			return nil, nil
		},
	}
	var out bytes.Buffer
	if err := runRole(context.Background(), r, ops, &out); err != nil { t.Fatal(err) }
	if !stage.closed || len(paths) != 3 || paths[0] != pdfinfoPath+" "+pdfPath ||
		paths[1] != pdftotextPath+" -enc UTF-8 -nopgbrk "+pdfPath+" /out/pilot-"+r.attempt+"/pilot.txt" ||
		paths[2] != pdftoppmPath+" -f 1 -l 1 -singlefile -r 72 -png "+pdfPath+" /out/pilot-"+r.attempt+"/page" {
		t.Fatalf("fixed reader order/arguments differ: %v", paths)
	}
	values := decodeTestFrame(t, out.Bytes(), r, 'D', recordText, recordPNG)
	if !bytes.Equal(values[0], stage.files["pilot.txt"]) || !bytes.Equal(values[1], stage.files["page.png"]) { t.Fatal("frame changed accepted original bytes") }
}

func TestPDFPilotDriverFailedPageCountAndCancelCannotEmitResult(t *testing.T) {
	pdf := []byte("fixed PDF source fixture")
	digest := sha256.Sum256(pdf)
	r := frameFixture(roleRead)
	r.inputDigest = hex.EncodeToString(digest[:])
	for _, cancelAfterInfo := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		stage := &inertPilotStage{files: map[string][]byte{"pilot.txt": []byte("日本語\n"), "page.png": []byte("invalid")}}
		calls := 0
		ops := driverOps{
			readInput: func(string, int) ([]byte, error) { return pdf, nil },
			newStage: func(string) (pilotStage, error) { return stage, nil },
			runTool: func(context.Context, string, []string, int) ([]byte, error) {
				calls++
				if cancelAfterInfo { cancel(); return []byte("Pages: 1\n"), nil }
				return []byte("Pages: 2\n"), nil
			},
		}
		var out bytes.Buffer
		if err := runRole(ctx, r, ops, &out); err == nil || out.Len() != 0 || calls != 1 || !stage.closed {
			t.Fatal("page-count refusal or cancellation progressed to another tool/result")
		}
		cancel()
	}
}

func TestPDFPilotDriverDoesNotRetainNativeOutput(t *testing.T) {
	stdout := &limitedOutput{limit: 0}
	if _, err := stdout.Write([]byte("raw tool text")); err == nil || len(stdout.data) != 0 { t.Fatal("tool text accepted") }
	stderr := &limitedOutput{limit: 4 << 10}
	if _, err := io.Copy(stderr, strings.NewReader("raw tool error")); err != nil || stderr.count == 0 || len(stderr.data) != 0 { t.Fatal("stderr was retained or not counted") }
}
