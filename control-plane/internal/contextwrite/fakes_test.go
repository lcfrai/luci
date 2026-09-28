package contextwrite_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// The brief ids the tests use. Real brief ids (provenance's brief form).
const (
	idA = "bf-00000000000000a1"
	idB = "bf-00000000000000b2"
	idP = "bf-00000000000000c7" // a project brief a branch brief pins
)

// fakeGit answers Tracked per rel path and records every question. Ignored
// answers true unless notIgnored names the path.
type fakeGit struct {
	mu           sync.Mutex
	tracked      map[string]bool
	trackedErr   error // returned by every Tracked call when set
	excludePath  string
	excludeErr   error
	notIgnored   map[string]bool
	ignoredErr   error
	onExclude    func() // runs inside ExcludePath: between step 5's plan and step 7's writes
	trackedCalls []string
	excludeCalls int
	ignoredCalls []string
}

func (g *fakeGit) Ignored(_ context.Context, _ string, rel string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.ignoredCalls = append(g.ignoredCalls, rel)
	if g.ignoredErr != nil {
		return false, g.ignoredErr
	}
	return !g.notIgnored[rel], nil
}

func (g *fakeGit) ignoredAsked() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.ignoredCalls...)
}

func (g *fakeGit) Tracked(_ context.Context, _ string, rel string) (bool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.trackedCalls = append(g.trackedCalls, rel)
	if g.trackedErr != nil {
		return false, g.trackedErr
	}
	return g.tracked[rel], nil
}

// ExcludePath answers excludePath, or dir's own .git/info/exclude when
// excludePath is "".
func (g *fakeGit) ExcludePath(_ context.Context, dir string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.excludeCalls++
	if g.onExclude != nil {
		g.onExclude()
	}
	if g.excludeErr != nil {
		return "", g.excludeErr
	}
	if g.excludePath == "" {
		return filepath.Join(dir, ".git", "info", "exclude"), nil
	}
	return g.excludePath, nil
}

// reset forgets the recorded calls.
func (g *fakeGit) reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.trackedCalls, g.excludeCalls, g.ignoredCalls = nil, 0, nil
}

// askedBothOnce: Tracked was asked of each candidate exactly once.
func (g *fakeGit) askedBothOnce(t *testing.T) {
	t.Helper()
	g.mu.Lock()
	defer g.mu.Unlock()
	n := map[string]int{}
	for _, rel := range g.trackedCalls {
		n[rel]++
	}
	if len(g.trackedCalls) != 2 || n[contextwrite.FileName] != 1 || n[contextwrite.AltFileName] != 1 {
		t.Fatalf("Tracked calls = %q, want %q and %q once each", g.trackedCalls, contextwrite.FileName, contextwrite.AltFileName)
	}
}

// fakeSource is discoverhub.Service's Inheritable, scripted.
type fakeSource struct {
	mu    sync.Mutex
	in    discoverhub.Inheritable
	err   error
	fn    func(ctx context.Context, t discoverhub.Target) (discoverhub.Inheritable, error)
	calls []discoverhub.Target
}

func (s *fakeSource) Inheritable(ctx context.Context, t discoverhub.Target) (discoverhub.Inheritable, error) {
	s.mu.Lock()
	s.calls = append(s.calls, t)
	fn, in, err := s.fn, s.in, s.err
	s.mu.Unlock()
	if fn != nil {
		return fn(ctx, t)
	}
	return in, err
}

func (s *fakeSource) set(in discoverhub.Inheritable, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.in, s.err = in, err
}

func (s *fakeSource) called() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// doc is a branch brief's document with one answered section.
func doc(id string, rev int, purpose string) discover.Document {
	return discover.Document{
		Title:    "Branch brief — evidence-upload",
		BriefID:  id,
		Revision: rev,
		Key:      provenance.BriefKey(id, rev),
		Sections: []discover.Section{{
			Heading: "Purpose",
			Body:    purpose,
			Items:   []discover.Criterion{},
			Origin:  discover.OriginBranch,
		}},
	}
}

func inh(d discover.Document) discoverhub.Inheritable {
	return discoverhub.Inheritable{Key: d.Key, Document: d}
}

// env is one worktree, a fake Git over it, a fake Source and a Materialiser.
type env struct {
	t   *testing.T
	dir string
	git *fakeGit
	src *fakeSource
	m   *contextwrite.Materialiser
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dir := t.TempDir()
	g := &fakeGit{tracked: map[string]bool{}, excludePath: filepath.Join(dir, ".git", "info", "exclude")}
	s := &fakeSource{}
	m := contextwrite.New(s, g, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &env{t: t, dir: dir, git: g, src: s, m: m}
}

func (e *env) req() api.MaterialiseRequest {
	return api.MaterialiseRequest{WorktreeID: "wt-1", WorktreePath: e.dir, ProjectID: "pr-1", BranchID: "br-1"}
}

func (e *env) path(rel string) string { return filepath.Join(e.dir, filepath.FromSlash(rel)) }

func (e *env) write(rel, content string) {
	e.t.Helper()
	p := e.path(rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

// read returns the file's bytes and whether it exists.
func (e *env) read(rel string) (string, bool) {
	e.t.Helper()
	b, err := os.ReadFile(e.path(rel))
	if errors.Is(err, os.ErrNotExist) {
		return "", false
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b), true
}

func (e *env) materialise(req api.MaterialiseRequest) (string, error) {
	res, err := e.m.Materialise(context.Background(), req)
	return res.ContextRef, err
}

// old is the moment every "untouched" file's mtime is set to.
var old = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// age sets rel's mtime to old, so a rewrite is visible.
func (e *env) age(rel string) {
	e.t.Helper()
	if err := os.Chtimes(e.path(rel), old, old); err != nil {
		e.t.Fatal(err)
	}
}

// untouched: rel holds want byte for byte and its mtime is still old.
func (e *env) untouched(rel, want string) {
	e.t.Helper()
	got, ok := e.read(rel)
	if !ok || got != want {
		e.t.Fatalf("%s = %q (exists %v), want it byte-identical: %q", rel, got, ok, want)
	}
	fi, err := os.Stat(e.path(rel))
	if err != nil {
		e.t.Fatal(err)
	}
	if !fi.ModTime().Equal(old) {
		e.t.Fatalf("%s was rewritten: mtime %v, want %v", rel, fi.ModTime(), old)
	}
}

func wantReason(t *testing.T, err error, reason string) {
	t.Helper()
	var me *api.MaterialiseError
	if !errors.As(err, &me) {
		t.Fatalf("err = %v, want *api.MaterialiseError{%s}", err, reason)
	}
	if me.Reason != reason {
		t.Fatalf("reason = %q, want %q (message %q)", me.Reason, reason, me.Message)
	}
	if me.Message == "" {
		t.Fatalf("reason %q has no message", reason)
	}
}

func wantPlain(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("err = nil, want a plain error")
	}
	var me *api.MaterialiseError
	if errors.As(err, &me) {
		t.Fatalf("err = %v, a refusal %q; want a plain error (a 500)", err, me.Reason)
	}
}

// block is a well-formed managed block for id@rev.
func block(id string, rev int, purpose string) string { return claudemd.Render(doc(id, rev, purpose)) }

// beginOnly is a begin marker with no end.
func beginOnly(id string, rev int) string {
	return claudemd.BeginPrefix + " brief=" + id + " rev=" + strconv.Itoa(rev) + " -->\n# A brief\n\nno end\n"
}

// excludeLines is the exclude file's lines, "" when it is absent.
func (e *env) excludeLines() []string {
	e.t.Helper()
	b, err := os.ReadFile(e.git.excludePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n"), "\n")
}

func count(lines []string, want string) int {
	n := 0
	for _, l := range lines {
		if l == want {
			n++
		}
	}
	return n
}
