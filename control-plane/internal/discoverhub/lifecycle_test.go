package discoverhub

import (
	"errors"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

func TestArchiveHeadLeavesKeysResolvable(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	if err := s.ArchiveHead(w.ctx, localProject, localBranch); err != nil {
		t.Fatalf("ArchiveHead: %v", err)
	}
	if h := w.headOf(w.b42); h.ArchivedAt == nil || h.HeadRevision != 6 {
		t.Fatalf("b-42 head = %+v, want archived at 6", h)
	}
	fb, err := w.hub.Fake.ForBranch(w.ctx, w.project, evidenceRef)
	if err != nil || fb.BranchBrief != nil {
		t.Fatalf("ForBranch after the archive = %+v, %v; want no live branch brief", fb.BranchBrief, err)
	}
	key := w.key(w.b42, 5)
	refs, _, _ := s.Resolve(w.ctx, []string{key})
	if len(refs) != 1 || refs[0].Resolution != provenance.Resolved {
		t.Fatalf("Resolve(%s) after the archive = %+v, want resolved", key, refs)
	}

	// Nothing to archive is nil: no brief (a second call finds none), the Hub
	// off, a project that is not shared.
	if err := s.ArchiveHead(w.ctx, localProject, localBranch); err != nil {
		t.Fatalf("ArchiveHead with no live brief = %v, want nil", err)
	}
	if err := w.service(withoutHub()).ArchiveHead(w.ctx, localProject, localBranch); err != nil {
		t.Fatalf("ArchiveHead with the Hub off = %v, want nil", err)
	}
	unbound := newBareWorld(t)
	unbound.locate.addProject(localProject, projectDir, "engagement-portal")
	unbound.repos.set(projectDir, gh("origin", teamKey))
	if err := unbound.service().ArchiveHead(unbound.ctx, localProject, localBranch); err != nil {
		t.Fatalf("ArchiveHead on an unbound project = %v, want nil", err)
	}
	if err := s.ArchiveHead(w.ctx, localProject, "b-unknown"); !errors.Is(err, errUnknownBranch) {
		t.Fatalf("ArchiveHead of a branch the Locator does not know = %v, want errUnknownBranch", err)
	}
}

func TestRebindMovesTheBrief(t *testing.T) {
	w := newWorld(t)
	s := w.service()
	if err := s.Rebind(w.ctx, localProject, "evidence-upload", "evidence-intake"); err != nil {
		t.Fatalf("Rebind: %v", err)
	}
	fb, err := w.hub.Fake.ForBranch(w.ctx, w.project, "refs/heads/evidence-intake")
	if err != nil || fb.BranchBrief == nil || fb.BranchBrief.Head.ID != w.b42 {
		t.Fatalf("ForBranch(evidence-intake) = %+v, %v; want b-42", fb.BranchBrief, err)
	}
	if fb, err := w.hub.Fake.ForBranch(w.ctx, w.project, evidenceRef); err != nil || fb.BranchBrief != nil {
		t.Fatalf("ForBranch(evidence-upload) = %+v, %v; want none", fb.BranchBrief, err)
	}

	// brief_exists on the new ref is the Hub's answer, unchanged.
	other := w.createBrief(w.project, hubapi.ScopeBranch, timeoutRef)
	err = s.Rebind(w.ctx, localProject, "evidence-intake", "fix-session-timeout")
	var he *hubclient.Error
	if !errors.As(err, &he) || he.Status != 409 || he.Problem.Error != hubapi.CodeBriefExists || he.Problem.Details["briefId"] != other {
		t.Fatalf("Rebind onto a held ref = %v, want the Hub's 409 brief_exists naming %s", err, other)
	}

	// Nothing to move is nil: no brief on the old name, the Hub off.
	if err := s.Rebind(w.ctx, localProject, "no-such-branch", "anything"); err != nil {
		t.Fatalf("Rebind with no brief = %v, want nil", err)
	}
	if err := w.service(withoutHub()).Rebind(w.ctx, localProject, "evidence-intake", "x"); err != nil {
		t.Fatalf("Rebind with the Hub off = %v, want nil", err)
	}
}
