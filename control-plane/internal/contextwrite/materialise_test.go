package contextwrite_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

func TestWritesTheInheritedKeyAndReturnsIt(t *testing.T) {
	e := newEnv(t)
	d := doc(idA, 6, "Upload evidence.")
	e.src.set(inh(d), nil)
	key, err := e.materialise(e.req())
	if err != nil {
		t.Fatal(err)
	}
	if key != d.Key {
		t.Fatalf("key = %q, want %q", key, d.Key)
	}
	got, ok := e.read(contextwrite.FileName)
	if !ok || got != claudemd.Render(d) {
		t.Fatalf("CLAUDE.md = %q (exists %v), want Render(doc)", got, ok)
	}
	if n := e.src.called(); n != 1 {
		t.Fatalf("Source asked %d times, want 1", n)
	}
	if got := e.src.calls[0]; got != (discoverhub.Target{ProjectID: "pr-1", BranchID: "br-1"}) {
		t.Fatalf("Source asked for %+v", got)
	}
}

func TestPreservesUserTextAroundTheBlock(t *testing.T) {
	for _, tc := range []struct{ name, before, after string }{
		{"text before only", "# Mine\n\nKeep this.\n", ""},
		{"text around an old block", "# Mine\n\n" + block(idA, 5, "old") + "\nAfter.\n", ""},
		{"crlf text", "# Mine\r\n\r\nKeep this.\r\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.write(contextwrite.FileName, tc.before)
			d := doc(idA, 6, "new")
			e.src.set(inh(d), nil)
			if _, err := e.materialise(e.req()); err != nil {
				t.Fatal(err)
			}
			got, _ := e.read(contextwrite.FileName)
			want, err := claudemd.Merge(tc.before, claudemd.Render(d))
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("CLAUDE.md = %q, want Merge(existing, Render) = %q", got, want)
			}
			b, ok, err := claudemd.Inspect(got)
			if err != nil || !ok {
				t.Fatalf("no block after the write: %v", err)
			}
			if outside := strings.Replace(got, b.Text, "", 1); !strings.Contains(outside, strings.SplitN(tc.before, "\n", 2)[0]) {
				t.Fatalf("user text lost: %q", outside)
			}
		})
	}
}

func TestTrackedClaudeMdFallsBackToTheAltFile(t *testing.T) {
	e := newEnv(t)
	e.git.tracked[contextwrite.FileName] = true
	e.write(contextwrite.FileName, "# The repository's own\n")
	e.age(contextwrite.FileName)
	d := doc(idA, 6, "Upload evidence.")
	e.src.set(inh(d), nil)
	key, err := e.materialise(e.req())
	if err != nil {
		t.Fatal(err)
	}
	if key != d.Key {
		t.Fatalf("key = %q", key)
	}
	got, ok := e.read(contextwrite.AltFileName)
	if !ok || got != claudemd.Render(d) {
		t.Fatalf(".claude/CLAUDE.md = %q (exists %v)", got, ok)
	}
	e.untouched(contextwrite.FileName, "# The repository's own\n")
	if n := count(e.excludeLines(), "/.claude/CLAUDE.md"); n != 1 {
		t.Fatalf("exclude = %q, want one /.claude/CLAUDE.md line", e.excludeLines())
	}
	if n := count(e.excludeLines(), "/CLAUDE.md"); n != 0 {
		t.Fatalf("exclude names the tracked CLAUDE.md: %q", e.excludeLines())
	}
}

func TestBothTrackedIsRefused(t *testing.T) {
	for _, without := range []bool{false, true} {
		t.Run(fmt.Sprintf("withoutBrief=%v", without), func(t *testing.T) {
			e := newEnv(t)
			e.git.tracked[contextwrite.FileName] = true
			e.git.tracked[contextwrite.AltFileName] = true
			e.src.set(inh(doc(idA, 6, "x")), nil)
			req := e.req()
			req.WithoutBrief = without
			_, err := e.materialise(req)
			wantReason(t, err, api.BriefReasonTracked)
			for _, rel := range []string{contextwrite.FileName, contextwrite.AltFileName} {
				if _, ok := e.read(rel); ok {
					t.Fatalf("%s written", rel)
				}
			}
			if _, err := os.Stat(e.git.excludePath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("exclude written: %v", err)
			}
			if n := e.src.called(); n != 0 {
				t.Fatalf("Source asked %d times", n)
			}
		})
	}
}

func TestExcludeLineIsAddedOnce(t *testing.T) {
	e := newEnv(t)
	earlier := "# git ls-files --others --exclude-from=.git/info/exclude\n*.log\n"
	if err := os.MkdirAll(filepath.Dir(e.git.excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.git.excludePath, []byte(earlier), 0o644); err != nil {
		t.Fatal(err)
	}
	for i, d := range []int{5, 6, 6} {
		e.src.set(inh(doc(idA, d, "rev")), nil)
		if _, err := e.materialise(e.req()); err != nil {
			t.Fatalf("materialise %d: %v", i, err)
		}
	}
	b, _ := os.ReadFile(e.git.excludePath)
	if !strings.HasPrefix(string(b), earlier) {
		t.Fatalf("exclude's earlier lines changed: %q", b)
	}
	if n := count(e.excludeLines(), "/CLAUDE.md"); n != 1 {
		t.Fatalf("exclude = %q, want exactly one /CLAUDE.md", b)
	}
	if n := count(e.excludeLines(), "*.log"); n != 1 {
		t.Fatalf("exclude = %q", b)
	}
}

func TestExcludeLineJoinsAFileWithNoFinalNewline(t *testing.T) {
	e := newEnv(t)
	if err := os.MkdirAll(filepath.Dir(e.git.excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.git.excludePath, []byte("*.log"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.src.set(inh(doc(idA, 6, "x")), nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(e.git.excludePath)
	if string(b) != "*.log\n/CLAUDE.md\n" {
		t.Fatalf("exclude = %q", b)
	}
}

// A CRLF exclude file (an editor's re-save on Windows) already holding the
// line is left alone: git ignores the trailing "\r", so the line is present,
// and a second copy per start would grow the file (mutation P2).
func TestExcludeLineInACRLFFileIsFound(t *testing.T) {
	e := newEnv(t)
	const crlf = "*.log\r\n/CLAUDE.md\r\n"
	if err := os.MkdirAll(filepath.Dir(e.git.excludePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(e.git.excludePath, []byte(crlf), 0o644); err != nil {
		t.Fatal(err)
	}
	e.src.set(inh(doc(idA, 6, "x")), nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(e.git.excludePath); string(b) != crlf {
		t.Fatalf("exclude = %q, want it unchanged: the line is already there", b)
	}
}

func TestExcludeFailureWritesNoBlock(t *testing.T) {
	for _, row := range []struct {
		name  string
		setup func(e *env)
		check func(t *testing.T, err error)
	}{
		{"exclude path under a regular file", func(e *env) {
			f := filepath.Join(e.t.TempDir(), "f")
			if err := os.WriteFile(f, []byte("a file"), 0o644); err != nil {
				e.t.Fatal(err)
			}
			e.git.excludePath = filepath.Join(f, "exclude")
		}, func(t *testing.T, err error) { wantReason(t, err, api.BriefReasonWriteFailed) }},
		{"ExcludePath errors", func(e *env) {
			e.git.excludeErr = errors.New("git: exploded")
		}, func(t *testing.T, err error) { wantPlain(t, err) }},
	} {
		for _, handWritten := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/handWritten=%v", row.name, handWritten), func(t *testing.T) {
				e := newEnv(t)
				row.setup(e)
				const mine = "# Mine\n\nKeep this.\n"
				if handWritten {
					e.write(contextwrite.FileName, mine)
					e.age(contextwrite.FileName)
				}
				e.src.set(inh(doc(idA, 6, "the brief")), nil)
				_, err := e.materialise(e.req())
				row.check(t, err)
				if handWritten {
					e.untouched(contextwrite.FileName, mine)
				} else if got, ok := e.read(contextwrite.FileName); ok {
					t.Fatalf("CLAUDE.md written: %q", got)
				}
				got, _ := e.read(contextwrite.FileName)
				if strings.Contains(got, claudemd.BeginPrefix) {
					t.Fatalf("a block landed in an un-excluded file: %q", got)
				}
			})
		}
	}
}

func TestNothingToInheritStripsAnOldBlock(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "Mine\n\n"+block(idA, 5, "old"))
	e.src.set(discoverhub.Inheritable{}, nil)
	key, err := e.materialise(e.req())
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Fatalf("key = %q, want \"\"", key)
	}
	if got, _ := e.read(contextwrite.FileName); got != "Mine\n" {
		t.Fatalf("CLAUDE.md = %q, want the block stripped", got)
	}
}

func TestAFileHoldingOnlyTheBlockIsRemovedOnStrip(t *testing.T) {
	for _, tc := range []struct{ name, content string }{
		{"the block alone", block(idA, 5, "old")},
		{"white space and the block", "\n\n" + block(idA, 5, "old")},
		{"crlf white space and the block", "\r\n \t\r\n" + block(idA, 5, "old")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.write(contextwrite.FileName, tc.content)
			e.src.set(discoverhub.Inheritable{}, nil)
			if _, err := e.materialise(e.req()); err != nil {
				t.Fatal(err)
			}
			if got, ok := e.read(contextwrite.FileName); ok {
				t.Fatalf("CLAUDE.md left behind: %q", got)
			}
		})
	}
}

func TestAWhiteSpaceFileWithNoBlockIsLeftAlone(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "\n\n")
	e.age(contextwrite.FileName)
	e.src.set(discoverhub.Inheritable{}, nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	e.untouched(contextwrite.FileName, "\n\n")
}

func TestWithoutBriefStripsAndDeclines(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "Mine\n\n"+block(idA, 5, "old"))
	e.write(contextwrite.AltFileName, block(idB, 2, "older"))
	e.src.set(discoverhub.Inheritable{}, fmt.Errorf("down: %w", hubclient.ErrUnavailable))
	req := e.req()
	req.WithoutBrief = true
	key, err := e.materialise(req)
	if err != nil {
		t.Fatal(err)
	}
	if key != "none:declined" {
		t.Fatalf("key = %q, want none:declined", key)
	}
	if k, err := provenance.Parse(key); err != nil || k.Kind != provenance.KindNone {
		t.Fatalf("%q is not a provenance none key: %v", key, err)
	}
	if got, _ := e.read(contextwrite.FileName); got != "Mine\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	if got, ok := e.read(contextwrite.AltFileName); ok {
		t.Fatalf(".claude/CLAUDE.md = %q, want it removed", got)
	}
	if n := e.src.called(); n != 0 {
		t.Fatalf("Source asked %d times", n)
	}
}

func TestNeverStripsATrackedFile(t *testing.T) {
	for _, tracked := range []struct{ name, content, markerKey string }{
		{"tracked block", "# Ours\n\n" + block(idB, 3, "committed"), provenance.BriefKey(idB, 3)},
		{"tracked begin marker only", "# Ours\n\n" + beginOnly(idB, 3), ""},
	} {
		for _, row := range []struct {
			name    string
			in      discoverhub.Inheritable
			without bool
			wantKey string
			wantAlt bool
		}{
			{"something to inherit", inh(doc(idA, 6, "new")), false, provenance.BriefKey(idA, 6), true},
			{"nothing to inherit", discoverhub.Inheritable{}, false, "", false},
			{"without the brief", inh(doc(idA, 6, "new")), true, "none:declined", false},
		} {
			t.Run(tracked.name+"/"+row.name, func(t *testing.T) {
				e := newEnv(t)
				e.git.tracked[contextwrite.FileName] = true
				e.write(contextwrite.FileName, tracked.content)
				e.age(contextwrite.FileName)
				e.src.set(row.in, nil)
				req := e.req()
				req.WithoutBrief = row.without
				key, err := e.materialise(req)
				if err != nil {
					t.Fatalf("err = %v, want success whatever the tracked file holds", err)
				}
				if key != row.wantKey {
					t.Fatalf("key = %q, want %q", key, row.wantKey)
				}
				e.untouched(contextwrite.FileName, tracked.content)
				got, ok := e.read(contextwrite.AltFileName)
				if row.wantAlt && (!ok || got != claudemd.Render(row.in.Document)) {
					t.Fatalf(".claude/CLAUDE.md = %q (exists %v)", got, ok)
				}
				if !row.wantAlt && ok {
					t.Fatalf(".claude/CLAUDE.md written: %q", got)
				}
				e.git.askedBothOnce(t)

				e.git.reset()
				ins, err := e.m.Inspect(context.Background(), req)
				if err != nil {
					t.Fatal(err)
				}
				e.git.askedBothOnce(t)
				if ins.Path != e.path(contextwrite.AltFileName) {
					t.Fatalf("Inspect Path = %q, want the target %q", ins.Path, e.path(contextwrite.AltFileName))
				}
				want := contextwrite.StateAbsent
				if row.wantAlt {
					want = contextwrite.StateCurrent
				}
				if ins.State != want {
					t.Fatalf("Inspect State = %q, want the target's own %q", ins.State, want)
				}
				if len(ins.Committed) != 1 || ins.Committed[0].Path != e.path(contextwrite.FileName) || ins.Committed[0].MarkerKey != tracked.markerKey {
					t.Fatalf("Committed = %+v, want one entry for the tracked CLAUDE.md, marker %q", ins.Committed, tracked.markerKey)
				}
			})
		}
	}
}

func TestHubUnavailableIsARefusal(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"ErrUnavailable", fmt.Errorf("hub: %w", hubclient.ErrUnavailable)},
		{"a pin the Hub cannot read", discoverhub.ErrPinUnreadable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			content := "Mine\n\n" + block(idA, 5, "old")
			e.write(contextwrite.FileName, content)
			e.age(contextwrite.FileName)
			e.src.set(discoverhub.Inheritable{}, tc.err)
			_, err := e.materialise(e.req())
			wantReason(t, err, api.BriefReasonHubUnavailable)
			e.untouched(contextwrite.FileName, content)
			if _, err := os.Stat(e.git.excludePath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("exclude written on a refusal: %v", err)
			}
			_, err = e.m.Inspect(context.Background(), e.req())
			wantReason(t, err, api.BriefReasonHubUnavailable)
		})
	}
}

// blackHole blocks until its ctx is done and returns the bare ctx.Err(),
// as a Hub read that never answers would.
func blackHole(ctx context.Context, _ discoverhub.Target) (discoverhub.Inheritable, error) {
	<-ctx.Done()
	return discoverhub.Inheritable{}, ctx.Err()
}

func TestABlackHoledHubIsARefusalWithinTheBudget(t *testing.T) {
	t.Run("the budget expires", func(t *testing.T) {
		e := newEnv(t)
		e.write(contextwrite.FileName, "Mine\n")
		e.age(contextwrite.FileName)
		e.src.fn = blackHole
		m := e.m.WithHubBudget(100 * time.Millisecond)
		start := time.Now()
		_, err := m.Materialise(context.Background(), e.req())
		if d := time.Since(start); d > time.Second {
			t.Fatalf("took %v, want the 100ms budget to end it", d)
		}
		wantReason(t, err, api.BriefReasonHubUnavailable)
		e.untouched(contextwrite.FileName, "Mine\n")
	})
	t.Run("the caller went away", func(t *testing.T) {
		e := newEnv(t)
		e.src.fn = blackHole
		m := e.m.WithHubBudget(10 * time.Second)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		time.AfterFunc(50*time.Millisecond, cancel)
		start := time.Now()
		_, err := m.Materialise(ctx, e.req())
		if d := time.Since(start); d > time.Second {
			t.Fatalf("took %v", d)
		}
		wantPlain(t, err)
		if _, ok := e.read(contextwrite.FileName); ok {
			t.Fatal("CLAUDE.md written")
		}
	})
}

func TestTheHubBudgetIsBelowTheDesktopSessionTimeout(t *testing.T) {
	const desktopSessionTimeout = 10 * time.Second // client.ts DEFAULT_TIMEOUT_MS
	if contextwrite.HubBudget >= desktopSessionTimeout {
		t.Fatalf("HubBudget = %v, want below the desktop's %v", contextwrite.HubBudget, desktopSessionTimeout)
	}
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "desktop", "src", "api", "client.ts"))
	if errors.Is(err, os.ErrNotExist) {
		t.Log("desktop/src/api/client.ts absent here; the literal is not cross-checked")
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "export const DEFAULT_TIMEOUT_MS = 10_000;") {
		t.Fatal("client.ts's DEFAULT_TIMEOUT_MS is no longer 10_000: recheck HubBudget against it")
	}
}

func TestSignedOutIsARefusal(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"ErrSignedOut", fmt.Errorf("refresh: %w", hubclient.ErrSignedOut)},
		{"a relayed 401", &hubclient.Error{Status: 401}},
		// The plan's b09 M-1: a tenant_not_enrolled sign-out lands here too.
		{"a relayed 403 tenant_not_enrolled", fmt.Errorf("list: %w", &hubclient.Error{Status: 403, Problem: httpx.Problem{Error: hubapi.CodeTenantNotEnrolled}})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.src.set(discoverhub.Inheritable{}, tc.err)
			_, err := e.materialise(e.req())
			wantReason(t, err, api.BriefReasonSignedOut)
		})
	}
}

func TestHubOffAndUnboundInheritNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"ErrHubOff", discoverhub.ErrHubOff},
		{"ErrNotBound", discoverhub.ErrNotBound},
		{"ErrNoRemote", discoverhub.ErrNoRemote},
		{"a Hub 404", &hubclient.Error{Status: 404}},
		{"none, no error", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.write(contextwrite.FileName, "Mine\n\n"+block(idA, 5, "old"))
			e.src.set(discoverhub.Inheritable{}, tc.err)
			key, err := e.materialise(e.req())
			if err != nil {
				t.Fatalf("err = %v, want no refusal", err)
			}
			if key != "" {
				t.Fatalf("key = %q", key)
			}
			if got, _ := e.read(contextwrite.FileName); got != "Mine\n" {
				t.Fatalf("CLAUDE.md = %q, want the old block stripped", got)
			}
		})
	}
}

func TestAnyOtherSourceErrorIsAPlainError(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"a plain error", errors.New("boom")},
		{"a Hub 409", &hubclient.Error{Status: 409}},
		{"a Hub 403 that is not tenant_not_enrolled", &hubclient.Error{Status: 403, Problem: httpx.Problem{Error: "forbidden"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.src.set(discoverhub.Inheritable{}, tc.err)
			_, err := e.materialise(e.req())
			wantPlain(t, err)
		})
	}
}

func TestNoProjectInheritsNothingWithoutAskingTheHub(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "Mine\n\n"+block(idA, 5, "old"))
	e.src.set(inh(doc(idA, 6, "x")), errors.New("Locator.ProjectPath(\"\"): unknown project"))
	req := e.req()
	req.ProjectID, req.BranchID = "", ""
	key, err := e.materialise(req)
	if err != nil {
		t.Fatal(err)
	}
	if key != "" {
		t.Fatalf("key = %q", key)
	}
	if got, _ := e.read(contextwrite.FileName); got != "Mine\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	if _, err := e.m.Inspect(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if n := e.src.called(); n != 0 {
		t.Fatalf("Source asked %d times, want never", n)
	}
}

func TestUnterminatedBlockIsBlockBroken(t *testing.T) {
	for _, tc := range []struct {
		name, rel string
		in        discoverhub.Inheritable
	}{
		{"the target, something to inherit", contextwrite.FileName, inh(doc(idA, 6, "new"))},
		{"the target, nothing to inherit", contextwrite.FileName, discoverhub.Inheritable{}},
		{"the other candidate, something to inherit", contextwrite.AltFileName, inh(doc(idA, 6, "new"))},
		{"a duplicate block", contextwrite.FileName, inh(doc(idA, 6, "new"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			content := "Mine\n\n" + beginOnly(idA, 5)
			if tc.name == "a duplicate block" {
				content = block(idA, 4, "one") + "\n" + block(idA, 5, "two")
			}
			e.write(tc.rel, content)
			e.age(tc.rel)
			e.src.set(tc.in, nil)
			_, err := e.materialise(e.req())
			wantReason(t, err, api.BriefReasonBlockBroken)
			e.untouched(tc.rel, content)
			if tc.rel == contextwrite.AltFileName {
				if got, ok := e.read(contextwrite.FileName); ok {
					t.Fatalf("the target was written before the other candidate's strip refused: %q", got)
				}
			}
		})
	}
}

func TestWithoutBriefOverAStrayEndMarkerIsBlockBroken(t *testing.T) {
	e := newEnv(t)
	content := "Mine\n\n" + claudemd.BeginPrefix + " brief=" + idA + " rev=5 -->\n# Brief\n" +
		claudemd.EndMarker + "\nthe brief's tail\n" + claudemd.EndMarker + "\n"
	e.write(contextwrite.FileName, content)
	e.age(contextwrite.FileName)
	req := e.req()
	req.WithoutBrief = true
	_, err := e.materialise(req)
	wantReason(t, err, api.BriefReasonBlockBroken)
	e.untouched(contextwrite.FileName, content)
	if n := e.src.called(); n != 0 {
		t.Fatalf("Source asked %d times", n)
	}
}

func TestWithoutBriefWriteErrorIsWriteFailed(t *testing.T) {
	e := newEnv(t)
	if err := os.Mkdir(e.path(contextwrite.FileName), 0o755); err != nil {
		t.Fatal(err)
	}
	req := e.req()
	req.WithoutBrief = true
	_, err := e.materialise(req)
	wantReason(t, err, api.BriefReasonWriteFailed)
	if n := e.src.called(); n != 0 {
		t.Fatalf("Source asked %d times", n)
	}
}

func TestATargetThatCannotBeReadIsWriteFailed(t *testing.T) {
	e := newEnv(t)
	if err := os.Mkdir(e.path(contextwrite.FileName), 0o755); err != nil {
		t.Fatal(err)
	}
	e.src.set(inh(doc(idA, 6, "new")), nil)
	_, err := e.materialise(e.req())
	wantReason(t, err, api.BriefReasonWriteFailed)
}

func TestUnchangedBytesAreNotRewritten(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "Mine\n")
	e.src.set(inh(doc(idA, 6, "same")), nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	first, _ := e.read(contextwrite.FileName)
	e.age(contextwrite.FileName)
	if err := os.Chtimes(e.git.excludePath, old, old); err != nil {
		t.Fatal(err)
	}
	exclude, _ := os.ReadFile(e.git.excludePath)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	e.untouched(contextwrite.FileName, first)
	fi, err := os.Stat(e.git.excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(e.git.excludePath); string(again) != string(exclude) || !fi.ModTime().Equal(old) {
		t.Fatalf("exclude rewritten with its line already present: %q, mtime %v", again, fi.ModTime())
	}
}

func TestFileIsWorldReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits")
	}
	e := newEnv(t)
	e.src.set(inh(doc(idA, 6, "x")), nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(e.path(contextwrite.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %o, want 0644", got)
	}
}

func TestConcurrentStartsOnOneWorktreeSerialise(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, "Mine\n")
	da, db := doc(idA, 6, "alpha"), doc(idB, 2, "beta")
	var mu sync.Mutex
	inflight, peak := 0, 0
	e.src.fn = func(_ context.Context, t discoverhub.Target) (discoverhub.Inheritable, error) {
		mu.Lock()
		inflight++
		peak = max(peak, inflight)
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		inflight--
		mu.Unlock()
		if t.BranchID == "a" {
			return inh(da), nil
		}
		return inh(db), nil
	}
	errs := make(chan error, 40)
	for i := 0; i < 20; i++ {
		for _, br := range []string{"a", "b"} {
			go func() {
				req := e.req()
				req.BranchID = br
				_, err := e.m.Materialise(context.Background(), req)
				errs <- err
			}()
		}
	}
	for i := 0; i < 40; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if peak != 1 {
		t.Fatalf("%d starts on one worktree overlapped, want them serialised", peak)
	}
	got, _ := e.read(contextwrite.FileName)
	if got != "Mine\n\n"+claudemd.Render(da) && got != "Mine\n\n"+claudemd.Render(db) {
		t.Fatalf("CLAUDE.md is not one complete render: %q", got)
	}
	if n := count(e.excludeLines(), "/CLAUDE.md"); n != 1 {
		t.Fatalf("exclude = %q", e.excludeLines())
	}
}

func TestStartsOnDifferentWorktreesRunInParallel(t *testing.T) {
	e1, e2 := newEnv(t), newEnv(t)
	entered := make(chan string, 2)
	release := make(chan struct{})
	// One Source and one Materialiser for both worktrees.
	src := &fakeSource{fn: func(_ context.Context, t discoverhub.Target) (discoverhub.Inheritable, error) {
		entered <- t.BranchID
		<-release
		return inh(doc(idA, 6, "x")), nil
	}}
	// A Git whose exclude file is each clone's own, as real clones have.
	m := contextwrite.New(src, &fakeGit{tracked: map[string]bool{}}, nil)
	done := make(chan error, 2)
	for _, e := range []*env{e1, e2} {
		req := e.req()
		req.BranchID = e.dir
		go func() {
			_, err := m.Materialise(context.Background(), req)
			done <- err
		}()
	}
	timeout := time.After(5 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-timeout:
			close(release)
			t.Fatal("the second worktree's start waited for the first: calls are serialised across worktrees")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestTheWorktreePathMustBeAnAbsoluteDirectory(t *testing.T) {
	e := newEnv(t)
	file := filepath.Join(e.dir, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, path string }{
		{"relative", "worktree"},
		{"missing", filepath.Join(e.dir, "gone")},
		{"a file", file},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := e.req()
			req.WorktreePath = tc.path
			_, err := e.materialise(req)
			wantPlain(t, err)
			_, err = e.m.Inspect(context.Background(), req)
			wantPlain(t, err)
			e.git.mu.Lock()
			n := len(e.git.trackedCalls)
			e.git.mu.Unlock()
			if n != 0 {
				t.Fatalf("git asked about a path that is not a worktree directory")
			}
		})
	}
}

func TestAGitErrorIsAPlainErrorNeverUntracked(t *testing.T) {
	e := newEnv(t)
	e.git.trackedErr = errors.New("git ls-files: exit status 128")
	e.src.set(inh(doc(idA, 6, "x")), nil)
	_, err := e.materialise(e.req())
	wantPlain(t, err)
	if _, ok := e.read(contextwrite.FileName); ok {
		t.Fatal("CLAUDE.md written on a guess at untracked")
	}
	_, err = e.m.Inspect(context.Background(), e.req())
	wantPlain(t, err)
}

func TestOutsideGitWritesNoExcludeLine(t *testing.T) {
	e := newEnv(t)
	e.git.trackedErr = fmt.Errorf("git: %w", contextwrite.ErrNotARepo)
	e.write(contextwrite.AltFileName, block(idB, 2, "older"))
	d := doc(idA, 6, "x")
	e.src.set(inh(d), nil)
	key, err := e.materialise(e.req())
	if err != nil {
		t.Fatal(err)
	}
	if key != d.Key {
		t.Fatalf("key = %q", key)
	}
	if got, _ := e.read(contextwrite.FileName); got != claudemd.Render(d) {
		t.Fatalf("CLAUDE.md = %q", got)
	}
	if got, ok := e.read(contextwrite.AltFileName); ok {
		t.Fatalf("the other candidate kept its block: %q", got)
	}
	if e.git.excludeCalls != 0 {
		t.Fatal("ExcludePath asked outside a repository")
	}
}

func TestARelativeExcludePathIsJoinedToTheWorktree(t *testing.T) {
	e := newEnv(t)
	e.git.excludePath = filepath.Join(".git", "info", "exclude")
	e.src.set(inh(doc(idA, 6, "x")), nil)
	if _, err := e.materialise(e.req()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(e.dir, ".git", "info", "exclude"))
	if err != nil || string(b) != "/CLAUDE.md\n" {
		t.Fatalf("exclude = %q, %v", b, err)
	}
}
