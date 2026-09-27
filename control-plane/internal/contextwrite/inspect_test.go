package contextwrite_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/contextwrite"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// handEdited is render with its purpose line changed, as a person's editor
// would leave it.
func handEdited(render string) string {
	out := strings.Replace(render, "Upload evidence.", "Upload evidence, by hand.", 1)
	if out == render {
		panic("the fixture's purpose line is not in the render")
	}
	return out
}

// badMarker is render with its begin line's revision spelled "06", which
// claudemd.Inspect refuses as ErrBadMarker while Merge and Strip, which only
// count markers, still see one well-placed block.
func badMarker(render string) string {
	out := strings.Replace(render, " rev=6 -->", " rev=06 -->", 1)
	if out == render {
		panic("the fixture's begin marker is not rev=6")
	}
	return out
}

func (e *env) inspect(req api.MaterialiseRequest) contextwrite.Inspection {
	e.t.Helper()
	ins, err := e.m.Inspect(context.Background(), req)
	if err != nil {
		e.t.Fatalf("Inspect: %v", err)
	}
	return ins
}

// wantEmptyLists: Diff and Committed are [] (not nil), so they marshal as [].
func wantEmptyLists(t *testing.T, ins contextwrite.Inspection, diffToo bool) {
	t.Helper()
	b, err := json.Marshal(ins)
	if err != nil {
		t.Fatal(err)
	}
	if ins.Committed == nil || len(ins.Committed) != 0 || !strings.Contains(string(b), `"committed":[]`) {
		t.Fatalf("Committed = %#v, want [] (json %s)", ins.Committed, b)
	}
	if diffToo && (ins.Diff == nil || len(ins.Diff) != 0 || !strings.Contains(string(b), `"diff":[]`)) {
		t.Fatalf("Diff = %#v, want [] (json %s)", ins.Diff, b)
	}
}

// hasLine: diff holds a line of op whose text contains sub.
func hasLine(diff []claudemd.DiffLine, op claudemd.DiffOp, sub string) bool {
	for _, l := range diff {
		if l.Op == op && strings.Contains(l.Text, sub) {
			return true
		}
	}
	return false
}

func TestInspectStates(t *testing.T) {
	d := doc(idA, 6, "Upload evidence.")
	render := claudemd.Render(d)
	none := discoverhub.Inheritable{}
	for _, tc := range []struct {
		name      string
		content   string // "" = no file
		in        discoverhub.Inheritable
		state     contextwrite.WorktreeState
		markerKey string
	}{
		{"absent", "", none, contextwrite.StateAbsent, ""},
		{"absent, a file of the person's own", "# Mine\n", none, contextwrite.StateAbsent, ""},
		{"pending, no file", "", inh(d), contextwrite.StatePending, ""},
		{"pending, a file of the person's own", "# Mine\n", inh(d), contextwrite.StatePending, ""},
		{"current", "# Mine\n\n" + render, inh(d), contextwrite.StateCurrent, d.Key},
		{"current, re-saved with CRLF", strings.ReplaceAll("# Mine\n\n"+render, "\n", "\r\n"), inh(d), contextwrite.StateCurrent, d.Key},
		{"current, the final line ending dropped", strings.TrimSuffix(render, "\n"), inh(d), contextwrite.StateCurrent, d.Key},
		{"superseded", block(idA, 5, "old"), inh(d), contextwrite.StateSuperseded, provenance.BriefKey(idA, 5)},
		{"superseded by another brief", block(idB, 2, "other"), inh(d), contextwrite.StateSuperseded, provenance.BriefKey(idB, 2)},
		{"edited", "# Mine\n\n" + handEdited(render), inh(d), contextwrite.StateEdited, d.Key},
		{"edited, the begin marker", badMarker(render), inh(d), contextwrite.StateEdited, ""},
		{"stale", "# Mine\n\n" + block(idA, 5, "old"), none, contextwrite.StateStale, provenance.BriefKey(idA, 5)},
		{"stale, the begin marker edited", badMarker(render), none, contextwrite.StateStale, ""},
		{"broken, unterminated", "# Mine\n\n" + beginOnly(idA, 5), inh(d), contextwrite.StateBroken, ""},
		{"broken, a stray end", "# Mine\n" + claudemd.EndMarker + "\n", none, contextwrite.StateBroken, ""},
		{"broken, duplicated", block(idA, 4, "one") + "\n" + block(idA, 5, "two"), inh(d), contextwrite.StateBroken, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			if tc.content != "" {
				e.write(contextwrite.FileName, tc.content)
				e.age(contextwrite.FileName)
			}
			e.src.set(tc.in, nil)
			ins := e.inspect(e.req())
			if ins.State != tc.state {
				t.Fatalf("State = %q, want %q (%+v)", ins.State, tc.state, ins)
			}
			if ins.MarkerKey != tc.markerKey || ins.ExpectedKey != tc.in.Key {
				t.Fatalf("MarkerKey %q ExpectedKey %q, want %q and %q", ins.MarkerKey, ins.ExpectedKey, tc.markerKey, tc.in.Key)
			}
			if ins.WorktreeID != "wt-1" || ins.Path != e.path(contextwrite.FileName) {
				t.Fatalf("WorktreeID %q Path %q", ins.WorktreeID, ins.Path)
			}
			if ins.Excluded != "/CLAUDE.md" || ins.AltTracked {
				t.Fatalf("Excluded %q AltTracked %v", ins.Excluded, ins.AltTracked)
			}
			wantEmptyLists(t, ins, tc.state != contextwrite.StateEdited)
			if tc.state == contextwrite.StateEdited {
				if len(ins.Diff) == 0 {
					t.Fatal("edited with no diff")
				}
				if tc.markerKey != "" && (!hasLine(ins.Diff, claudemd.DiffAdded, "by hand") || !hasLine(ins.Diff, claudemd.DiffRemoved, "Upload evidence.")) {
					t.Fatalf("Diff = %+v, want the changed purpose line", ins.Diff)
				}
				if tc.markerKey == "" && !hasLine(ins.Diff, claudemd.DiffAdded, "rev=06") {
					t.Fatalf("Diff = %+v, want the edited marker line", ins.Diff)
				}
			}
			// Inspect reads, never writes.
			if tc.content != "" {
				e.untouched(contextwrite.FileName, tc.content)
			} else if _, ok := e.read(contextwrite.FileName); ok {
				t.Fatal("Inspect wrote CLAUDE.md")
			}
			if _, err := os.Stat(e.git.excludePath); !os.IsNotExist(err) {
				t.Fatalf("Inspect wrote the exclude file: %v", err)
			}
			if e.git.excludeCalls != 0 {
				t.Fatal("Inspect asked ExcludePath")
			}
		})
	}
}

// withProject is d resolved against a project revision: one section from
// the project, the erased text a pin erasure must never let the view show.
func withProject(d discover.Document) discover.Document {
	d.ProjectBriefID, d.ProjectRevision = idP, 5
	d.Sections = append([]discover.Section{{
		Heading: "Audience",
		Body:    "The erased project audience.",
		Items:   []discover.Criterion{},
		Origin:  discover.OriginProject,
	}}, d.Sections...)
	return d
}

// Re-review b06-2 R2-I1: after the org erases the project revision a branch
// revision pins, the same branch key renders its own text alone (task 07
// rule 7). A block written before the erasure is then not the key's render,
// but nobody edited it: it is superseded, with no diff, so neither the
// "edited by hand" warning nor the erased text reaches the desktop. The next
// start rewrites it.
func TestAnErasedPinIsSupersededNotEdited(t *testing.T) {
	own := doc(idA, 6, "Upload evidence.")
	before := withProject(own)
	if before.Key != own.Key {
		t.Fatal("fixture: one key, two renders")
	}
	for _, tc := range []struct {
		name   string
		erased bool
		state  contextwrite.WorktreeState
	}{
		{"the pin erased", true, contextwrite.StateSuperseded},
		// The control: the same bytes with PinErased unset are a hand edit.
		{"the pin not erased", false, contextwrite.StateEdited},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.write(contextwrite.FileName, "# Mine\n\n"+claudemd.Render(before))
			e.src.set(discoverhub.Inheritable{Key: own.Key, Document: own, PinErased: tc.erased}, nil)
			ins := e.inspect(e.req())
			if ins.State != tc.state || ins.MarkerKey != own.Key || ins.ExpectedKey != own.Key {
				t.Fatalf("State %q MarkerKey %q ExpectedKey %q, want %q and the key twice", ins.State, ins.MarkerKey, ins.ExpectedKey, tc.state)
			}
			if !tc.erased {
				return
			}
			wantEmptyLists(t, ins, true)
			b, err := json.Marshal(ins)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(b), "erased project audience") {
				t.Fatalf("the Inspection shows the erased text: %s", b)
			}
			// The next start rewrites the block with the own-text render.
			key, err := e.materialise(e.req())
			if err != nil || key != own.Key {
				t.Fatalf("Materialise = %q, %v", key, err)
			}
			if got, _ := e.read(contextwrite.FileName); got != "# Mine\n\n"+claudemd.Render(own) {
				t.Fatalf("CLAUDE.md = %q, want the own-text render", got)
			}
			if ins := e.inspect(e.req()); ins.State != contextwrite.StateCurrent {
				t.Fatalf("after the rewrite State = %q, want current", ins.State)
			}
		})
	}
}

func TestInspectListsCommittedBesideTheTargetsState(t *testing.T) {
	d := doc(idA, 6, "Upload evidence.")
	render := claudemd.Render(d)
	const ours = "# Ours\n\n"
	committed := ours + block(idB, 3, "committed")
	for _, tc := range []struct {
		name, tracked, target string
		state                 contextwrite.WorktreeState
		entries               []contextwrite.CommittedBlock // Path filled below
	}{
		{"target current", committed, render, contextwrite.StateCurrent, []contextwrite.CommittedBlock{{MarkerKey: provenance.BriefKey(idB, 3)}}},
		{"target edited", committed, handEdited(render), contextwrite.StateEdited, []contextwrite.CommittedBlock{{MarkerKey: provenance.BriefKey(idB, 3)}}},
		{"target broken", committed, beginOnly(idA, 5), contextwrite.StateBroken, []contextwrite.CommittedBlock{{MarkerKey: provenance.BriefKey(idB, 3)}}},
		{"target superseded", committed, block(idA, 5, "old"), contextwrite.StateSuperseded, []contextwrite.CommittedBlock{{MarkerKey: provenance.BriefKey(idB, 3)}}},
		{"target pending", committed, "", contextwrite.StatePending, []contextwrite.CommittedBlock{{MarkerKey: provenance.BriefKey(idB, 3)}}},
		{"tracked begin marker only", ours + beginOnly(idB, 3), render, contextwrite.StateCurrent, []contextwrite.CommittedBlock{{MarkerKey: ""}}},
		{"tracked stray end", ours + claudemd.EndMarker + "\n", render, contextwrite.StateCurrent, []contextwrite.CommittedBlock{{MarkerKey: ""}}},
		{"tracked edited marker", ours + badMarker(render), render, contextwrite.StateCurrent, []contextwrite.CommittedBlock{{MarkerKey: ""}}},
		{"tracked, no block", ours, render, contextwrite.StateCurrent, []contextwrite.CommittedBlock{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			e.git.tracked[contextwrite.FileName] = true
			e.write(contextwrite.FileName, tc.tracked)
			e.age(contextwrite.FileName)
			if tc.target != "" {
				e.write(contextwrite.AltFileName, tc.target)
			}
			e.src.set(inh(d), nil)
			ins := e.inspect(e.req())
			if ins.Path != e.path(contextwrite.AltFileName) || ins.State != tc.state {
				t.Fatalf("Path %q State %q, want the target %q in its own state %q", ins.Path, ins.State, e.path(contextwrite.AltFileName), tc.state)
			}
			if tc.state == contextwrite.StateEdited && !hasLine(ins.Diff, claudemd.DiffAdded, "by hand") {
				t.Fatalf("an edited target lost its diff behind the committed block: %+v", ins.Diff)
			}
			for i := range tc.entries {
				tc.entries[i].Path = e.path(contextwrite.FileName)
			}
			if fmt.Sprint(ins.Committed) != fmt.Sprint(tc.entries) || ins.Committed == nil {
				t.Fatalf("Committed = %+v, want %+v", ins.Committed, tc.entries)
			}
			if ins.Excluded != "/.claude/CLAUDE.md" {
				t.Fatalf("Excluded = %q", ins.Excluded)
			}
			e.untouched(contextwrite.FileName, tc.tracked)
		})
	}
	t.Run("both tracked", func(t *testing.T) {
		e := newEnv(t)
		e.git.tracked[contextwrite.FileName] = true
		e.git.tracked[contextwrite.AltFileName] = true
		e.write(contextwrite.FileName, committed)
		e.write(contextwrite.AltFileName, block(idA, 5, "also committed"))
		e.src.set(inh(d), nil)
		_, err := e.m.Inspect(context.Background(), e.req())
		wantReason(t, err, api.BriefReasonTracked)
		if n := e.src.called(); n != 0 {
			t.Fatalf("Source asked %d times", n)
		}
	})
}

func TestInspectReportsTheExcludeLine(t *testing.T) {
	for _, tc := range []struct {
		name          string
		tracked       map[string]bool
		notARepo      bool
		path, exclude string
		altTracked    bool
	}{
		{"both untracked", nil, false, contextwrite.FileName, "/CLAUDE.md", false},
		{"only .claude/CLAUDE.md tracked", map[string]bool{contextwrite.AltFileName: true}, false, contextwrite.FileName, "/CLAUDE.md", true},
		{"CLAUDE.md tracked", map[string]bool{contextwrite.FileName: true}, false, contextwrite.AltFileName, "/.claude/CLAUDE.md", false},
		{"not a repository", nil, true, contextwrite.FileName, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			for k, v := range tc.tracked {
				e.git.tracked[k] = v
			}
			if tc.notARepo {
				e.git.trackedErr = fmt.Errorf("git ls-files: %w", contextwrite.ErrNotARepo)
			}
			e.src.set(inh(doc(idA, 6, "x")), nil)
			ins := e.inspect(e.req())
			if ins.Path != e.path(tc.path) || ins.Excluded != tc.exclude || ins.AltTracked != tc.altTracked {
				t.Fatalf("Path %q Excluded %q AltTracked %v, want %q %q %v", ins.Path, ins.Excluded, ins.AltTracked, e.path(tc.path), tc.exclude, tc.altTracked)
			}
			if e.git.excludeCalls != 0 {
				t.Fatal("Inspect asked ExcludePath")
			}
			e.git.askedBothOnce(t)
		})
	}
}

func TestInspectWithoutBriefExpectsNothing(t *testing.T) {
	e := newEnv(t)
	e.write(contextwrite.FileName, block(idA, 5, "old"))
	e.src.set(inh(doc(idA, 6, "x")), nil)
	req := e.req()
	req.WithoutBrief = true
	ins := e.inspect(req)
	if ins.State != contextwrite.StateStale || ins.ExpectedKey != "" {
		t.Fatalf("State %q ExpectedKey %q, want stale and none", ins.State, ins.ExpectedKey)
	}
	if n := e.src.called(); n != 0 {
		t.Fatalf("Source asked %d times", n)
	}
}
