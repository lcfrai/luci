package contextwrite

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/claudemd"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discoverhub"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

type WorktreeState string

const (
	StateAbsent  WorktreeState = "absent"  // no block, and nothing to inherit
	StatePending WorktreeState = "pending" // no block yet; the next session start writes one
	StateCurrent WorktreeState = "current" // claudemd.SameText(block, render) for the key a start would write now: byte-exact but for CRLF vs LF (review M7) and the block's final line ending (task 05 As built 11)
	// StateSuperseded: the block names another key than the one a start
	// would write now; sessions already running read it. Also (re-review
	// b06-2 R2-I1) a block naming the right key whose text is not that key's
	// render because the project revision the key pins was erased since it
	// was written (Inheritable.PinErased): the key now renders its own text
	// alone (task 07 rule 7), nobody edited the file, and Diff stays [] so
	// the erased text is not shown. The file keeps the erased text until the
	// next start rewrites it (plan OQ-9).
	StateSuperseded WorktreeState = "superseded"
	// StateEdited: the marker names the right key, and the text is not that
	// key's render (state 11). A key's render depends on nothing but the key -
	// the title comes from the revision's own branchRef, never from a project
	// or branch name (task 05's Document.Title, review I5) - so a rename is
	// never "edited"; only a change to the file's bytes is, other than an
	// editor turning LF into CRLF (claudemd.SameText, review M7) or dropping
	// the line ending after the end marker (task 05 As built 11: SameText is
	// false exactly when DiffLines shows a change). The one exception is an
	// erased pin, above. Diff comes from claudemd.DiffLines, which splits on
	// \r?\n. A well-placed block whose begin line names no brief key
	// (claudemd.ErrBadMarker: the marker itself edited) is edited too, with
	// MarkerKey "", when a start would write a key, and stale when not: the
	// next start replaces or strips it like any other block.
	StateEdited WorktreeState = "edited"
	StateStale  WorktreeState = "stale"  // a block, but nothing should be inherited now
	StateBroken WorktreeState = "broken" // unterminated or duplicated: the next start is refused block_broken
)

// CommittedBlock is a managed block, or a marker of one, in a file git
// TRACKS. It is repository text: never written, never stripped (review I3).
// It is NOT a WorktreeState, because it describes a second file, and one
// State cannot describe two files without hiding one (re-review R-M1).
type CommittedBlock struct {
	Path      string `json:"path"`      // the tracked file, absolute
	MarkerKey string `json:"markerKey"` // its marker's key; "" when its markers are broken
}

type Inspection struct {
	WorktreeID  string              `json:"worktreeId"`
	Path        string              `json:"path"`        // the TARGET inspected, absolute
	State       WorktreeState       `json:"state"`       // the target's own state, always one of the seven above
	MarkerKey   string              `json:"markerKey"`   // "" when there is no block
	ExpectedKey string              `json:"expectedKey"` // "" when nothing should be inherited
	Diff        []claudemd.DiffLine `json:"diff"`        // StateEdited only; [] otherwise
	Committed   []CommittedBlock    `json:"committed"`   // every tracked candidate holding a block or marker, FileName first; [] when none
	// Excluded is the info/exclude line Materialise's step 6 adds before it
	// writes a block to this target: "/CLAUDE.md" or "/.claude/CLAUDE.md";
	// "" when git does not manage the directory (ErrNotARepo, step 2). It
	// comes from the target choice alone, with no git call and no read of
	// the exclude file. Task 19 says with it that git stops listing the file,
	// text of the person's own in it included (review M9).
	Excluded string `json:"excluded"`
	// AltTracked is step 2's Git.Tracked answer for AltFileName
	// (.claude/CLAUDE.md), kept, not asked again. It is false when git does
	// not manage the directory. Task 19 needs it: when the target is
	// FileName and AltTracked is true, committing CLAUDE.md makes both
	// candidates tracked, and step 2 then refuses every start in this
	// worktree (tracked_elsewhere, review N1), so the view must not offer
	// that commit as a way back (re-review b09 I-1).
	AltTracked bool `json:"altTracked"`
}

// Committed holds at most one entry today: an Inspection is returned only
// when one candidate is the untracked target, so at most the other is
// tracked. It is a list, ordered, so review N1's batch can report both
// files of a both-tracked repository without a shape change.

// Inspect reads, never writes. It is the ONLY place brief text is read back
// out of a worktree, and it reads it only to compare (contract §3.3).
//
// It asks Git.Tracked of both candidates, as Materialise's step 2 does, and
// chooses the target the same way; it asks Source as step 4 does (ProjectID
// "" and WithoutBrief ask nothing and expect none; the same budget; the same
// refusals, as *api.MaterialiseError, and the same "nothing to inherit").
// The target is inspected for its own
// state (the seven above), always, so a committed block elsewhere never
// hides an edited, broken or superseded target: state 11's diff and its
// rewrite warning still show, and a broken target still names itself
// (re-review R-M1). Separately, every TRACKED candidate that holds a block,
// or any marker of one (claudemd.Inspect ok, or ErrUnterminatedBlock /
// ErrDuplicateBlock / ErrBadMarker), is listed in Committed, FileName before
// AltFileName, with its marker's key ("" when its markers are broken).
// Committed is [] when there is none. With both candidates tracked there is
// no target, and Inspect answers what the shared target choice answers
// (tracked_elsewhere) whatever the files hold: no Panacea session starts in
// that repository (Materialise refuses every start there, WithoutBrief
// included, which is review N1, open for its own batch), and the refusal
// names the cause a person acts on. N1's batch revisits Committed on that
// path. A candidate that cannot be read is a plain error.
func (m *Materialiser) Inspect(ctx context.Context, req api.MaterialiseRequest) (Inspection, error) {
	dir, err := worktreeDir(req.WorktreePath)
	if err != nil {
		return Inspection{}, err
	}
	c, err := m.choose(ctx, dir)
	if err != nil {
		return Inspection{}, err
	}
	var in discoverhub.Inheritable
	if !req.WithoutBrief {
		if in, err = m.inherit(ctx, req); err != nil {
			return Inspection{}, err
		}
	}
	target := c.abs(c.target)
	content, _, err := readForInspect(dir, target)
	if err != nil {
		return Inspection{}, err
	}
	ins := Inspection{
		WorktreeID:  req.WorktreeID,
		Path:        target,
		ExpectedKey: in.Key,
		Diff:        []claudemd.DiffLine{},
		Committed:   []CommittedBlock{},
		Excluded:    c.excludeLine(),
		AltTracked:  c.repo && c.tracked[1],
	}
	ins.State, ins.MarkerKey, ins.Diff = stateOf(content, in)
	for i, rel := range candidates {
		if !c.tracked[i] {
			continue
		}
		p := c.abs(rel)
		content, ok, err := readForInspect(dir, p)
		if err != nil {
			return Inspection{}, err
		}
		if !ok {
			continue
		}
		b, found, err := claudemd.Inspect(content)
		switch {
		case err == nil && found:
			key, _ := markerKey(b.Marker)
			ins.Committed = append(ins.Committed, CommittedBlock{Path: p, MarkerKey: key})
		case errors.Is(err, claudemd.ErrUnterminatedBlock), errors.Is(err, claudemd.ErrDuplicateBlock),
			errors.Is(err, claudemd.ErrBadMarker):
			ins.Committed = append(ins.Committed, CommittedBlock{Path: p})
		case err != nil:
			return Inspection{}, fmt.Errorf("contextwrite: inspect %s: %w", p, err)
		}
	}
	return ins, nil
}

// readForInspect is readCandidate with a plain error: Inspect refuses
// nothing, it only reports. The link rule (confine.go) holds here too, so no
// host file is read through a link into an Inspection's Diff.
func readForInspect(dir, path string) (string, bool, error) {
	s, ok, err := readChecked(dir, path)
	if err != nil {
		return "", false, fmt.Errorf("contextwrite: inspect: %w", err)
	}
	return s, ok, nil
}

// stateOf is the target's own state against what a start would write now.
func stateOf(content string, in discoverhub.Inheritable) (WorktreeState, string, []claudemd.DiffLine) {
	none := []claudemd.DiffLine{}
	b, found, err := claudemd.Inspect(content)
	var text, key string
	switch {
	case errors.Is(err, claudemd.ErrBadMarker):
		// The markers are well placed (Merge and Strip accept them) but the
		// begin line names no key: the marker itself was edited.
		text = badMarkerBlock(content)
	case err != nil:
		return StateBroken, "", none
	case !found:
		if in.Key == "" {
			return StateAbsent, "", none
		}
		return StatePending, "", none
	default:
		text = b.Text
		var ok bool
		if key, ok = markerKey(b.Marker); !ok {
			key = "" // a MarkerSafe id that is no brief id: an edited marker too
		}
	}
	switch {
	case in.Key == "":
		return StateStale, key, none
	case key != "" && key != in.Key:
		return StateSuperseded, key, none
	}
	render := claudemd.Render(in.Document)
	switch {
	case key != "" && claudemd.SameText(text, render):
		return StateCurrent, key, none
	case key != "" && in.PinErased:
		return StateSuperseded, key, none
	}
	return StateEdited, key, claudemd.DiffLines(render, text)
}

// markerKey is the brief key a begin marker names, when it names one.
// provenance.BriefKey panics on an id that is not a brief id, and a
// hand-edited marker may name one, so the key is parsed, not built.
func markerKey(mk claudemd.Marker) (string, bool) {
	s := "brief:" + mk.BriefID + "@" + strconv.Itoa(mk.Revision)
	k, err := provenance.Parse(s)
	if err != nil || k.Kind != provenance.KindBrief {
		return "", false
	}
	return s, true
}

// badMarkerBlock is the block text of a file claudemd.Inspect refused with
// ErrBadMarker. That error comes only after claudemd's marker count passed
// (exactly one begin, then exactly one end), so the first BeginPrefix and
// the first EndMarker bound the block; the count itself stays claudemd's.
func badMarkerBlock(content string) string {
	i := strings.Index(content, claudemd.BeginPrefix)
	j := strings.Index(content, claudemd.EndMarker)
	if i < 0 || j < i {
		return content
	}
	return content[i : j+len(claudemd.EndMarker)]
}
