package discoverhub

import (
	"context"
	"errors"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// ArchiveHead archives the branch's live brief when the branch is deleted
// (contract P13; phase 3's briefref.Archiver, ask P4-3). Revisions are
// untouched and every key still resolves. No brief, Hub off, or project not
// bound: nil - there is nothing to archive. A brief archived by someone else
// between the read and the call (brief_archived) is nil too.
//
// As built: a branch id the Locator does not know is errUnknownBranch, not
// nil, because nothing can be looked up and the brief would stay live
// unseen; the caller archives before the branch leaves the Locator.
func (s *Service) ArchiveHead(ctx context.Context, projectID, branchID string) error {
	if s.cfg.Hub == nil {
		return nil
	}
	ref := s.BranchRef(branchID)
	if ref == "" {
		return errUnknownBranch
	}
	hp, ok, err := s.boundOrNothing(ctx, projectID)
	if err != nil || !ok {
		return err
	}
	fb, err := s.cfg.Hub.ForBranch(ctx, hp, ref)
	if err != nil || fb.BranchBrief == nil {
		return err
	}
	_, err = s.cfg.Hub.Archive(ctx, fb.BranchBrief.Head.ID)
	if hubclient.Code(err) == hubapi.CodeBriefArchived {
		return nil
	}
	return err
}

// Rebind moves the branch's live brief to the renamed ref (Hub PATCH
// /v1/briefs/{id}); seam S4-P3.6, called by phase 3's RenameBranch. It takes
// NAMES, because after a rename the old branch id may no longer resolve.
// brief_exists on the new ref is returned unchanged: two briefs, one ref,
// is the person's to settle, not ours to merge. No brief on the old name,
// Hub off, or project not bound: nil - there is nothing to move.
func (s *Service) Rebind(ctx context.Context, projectID, oldBranchName, newBranchName string) error {
	if s.cfg.Hub == nil || oldBranchName == newBranchName {
		return nil
	}
	if oldBranchName == "" || newBranchName == "" {
		return errUnknownBranch
	}
	hp, ok, err := s.boundOrNothing(ctx, projectID)
	if err != nil || !ok {
		return err
	}
	fb, err := s.cfg.Hub.ForBranch(ctx, hp, "refs/heads/"+oldBranchName)
	if err != nil || fb.BranchBrief == nil {
		return err
	}
	_, err = s.cfg.Hub.Rebind(ctx, fb.BranchBrief.Head.ID, "refs/heads/"+newBranchName)
	return err
}

// boundOrNothing is projectID's Hub project; ok false with no error when the
// project is not shared (unbound, or no recognised remote): nothing of it
// lives in the Hub. Any other error is returned unchanged.
func (s *Service) boundOrNothing(ctx context.Context, projectID string) (string, bool, error) {
	b, err := s.bound(ctx, projectID)
	switch {
	case errors.Is(err, ErrNotBound), errors.Is(err, ErrNoRemote), errors.Is(err, ErrHubOff):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return b.HubProjectID, true, nil
}
