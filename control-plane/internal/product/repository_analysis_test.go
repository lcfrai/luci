package product

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func analysisWrite(t *testing.T, root, path, text string) {
	t.Helper()
	target := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRepositoryAnalysisRealGitSnapshotExcludesPrivateIgnoredAndLinksWithoutWriting(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "analysis-app")
	analysisWrite(t, p.Path, "README.md", "An app that saves a customer's delivery.")
	analysisWrite(t, p.Path, "src/app.ts", "export const saveDelivery = () => true;")
	analysisWrite(t, p.Path, ".gitignore", "ignored.ts\ntracked-private.ts\n")
	for _, path := range []string{"ignored.ts", "tracked-private.ts", ".env", "runtime.env", "secrets/config.json", "private/data.json", ".luci/provider.json", "data/accounts.json"} {
		analysisWrite(t, p.Path, path, "private-content-must-not-cross")
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "add", "--force", "tracked-private.ts"); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "private.ts")
	analysisWrite(t, filepath.Dir(outside), filepath.Base(outside), "private-external-link")
	// Windows may not allow unprivileged symlinks; the mandatory hardlink case
	// verifies that an ordinary-looking second name is excluded before reading.
	if err := os.Link(outside, filepath.Join(p.Path, "hardlinked.ts")); err != nil {
		t.Fatal(err)
	}
	_ = os.Symlink(outside, filepath.Join(p.Path, "linked.ts"))
	before, _ := s.journeyGit(context.Background(), p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	snapshot, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := RepositoryAnalysisPrompt(snapshot, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-content-must-not-cross", "private-external-link", "tracked-private.ts", "ignored.ts", "runtime.env", "hardlinked.ts", "provider.json"} {
		if strings.Contains(string(prompt), private) {
			t.Fatalf("protected source crossed snapshot: %s", private)
		}
	}
	if !strings.Contains(string(prompt), "saveDelivery") || snapshot.Source.FileCount < 2 || !snapshot.Source.Partial || len(snapshot.Source.Digest) != 64 {
		t.Fatal("actual public source or truthful receipt missing")
	}
	for _, file := range snapshot.Source.Files {
		if len(file.Digest) != 64 || file.Bytes > analysisFileBytes {
			t.Fatal("invalid source receipt")
		}
	}
	after, _ := s.journeyGit(context.Background(), p.Path, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if before != after {
		t.Fatal("analysis wrote/staged source")
	}
	if _, err := s.RepositoryAnalysisSnapshot(context.Background(), "p-other", "worktree"); err == nil {
		t.Fatal("unowned project was analysed")
	}
	if _, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "commit"); err == nil {
		t.Fatal("unborn HEAD fabricated committed source")
	}
}

func TestRepositoryAnalysisCommittedBytesAndBoundedInventoryHaveExactProvenance(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "analysis-bounds")
	analysisWrite(t, p.Path, "README.md", "Committed purpose")
	analysisWrite(t, p.Path, "src/app.ts", "export const version = 1;")
	analysisWrite(t, p.Path, "huge.ts", strings.Repeat("x", analysisFileBytes+1))
	analysisWrite(t, p.Path, "binary.ts", "binary\x00data")
	for i := 0; i < 70; i++ {
		analysisWrite(t, p.Path, fmt.Sprintf("src/f%03d.ts", i), "export const ok = true;")
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "add", "README.md", "src"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.journeyGit(context.Background(), p.Path, "-c", "user.name=Luci fixture", "-c", "user.email=fixture@example.test", "commit", "-m", "Actual source fixture"); err != nil {
		t.Fatal(err)
	}
	analysisWrite(t, p.Path, "src/app.ts", "export const version = 2;")
	committed, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "commit")
	if err != nil {
		t.Fatal(err)
	}
	working, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	if committed.Source.Commit == "" || committed.Source.Digest == working.Source.Digest || working.Source.FileCount > 60 || working.Source.IncludedBytes > analysisTotalBytes || working.Source.SkippedFiles < 12 {
		t.Fatal("mode or bounded coverage provenance lost")
	}
	for _, file := range committed.Files {
		if file.Path == "src/app.ts" && !strings.Contains(file.Content, "version = 1") {
			t.Fatal("commit analysis read worktree bytes")
		}
	}
	before := working.Source.Digest
	analysisWrite(t, p.Path, "src/new.ts", "export const added = true;")
	changed, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil || changed.Source.Digest == before {
		t.Fatal("eligible inventory changes outside sample were missed")
	}
}

func TestRepositoryAnalysisApplyChecksSourceAndBothRevisionsAndOnlySavesDraft(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "analysis-review")
	analysisWrite(t, p.Path, "README.md", "Delivery app")
	analysisWrite(t, p.Path, "app.ts", "export const deliveries = [];")
	snapshot, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	proposal := snapshot.Specification
	proposal.StartingPoint = "existing"
	proposal.Purpose = "Save deliveries"
	proposal.Audience = "Customers"
	proposal.Requirements = []SpecificationRequirement{{ID: "AC-delivery", Title: "Save a delivery", Priority: "must", Acceptance: "Reloading restores the saved delivery"}}
	analysisWrite(t, p.Path, "app.ts", "export const deliveries = [1];")
	if _, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, 0, 0, "worktree", snapshot.Source.Digest, proposal); !errors.Is(err, ErrAnalysisChanged) {
		t.Fatal("changed source was applied", err)
	}
	analysisWrite(t, p.Path, "app.ts", "export const deliveries = [];")
	if _, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, 1, 0, "worktree", snapshot.Source.Digest, proposal); !errors.Is(err, ErrAnalysisChanged) {
		t.Fatal("stale specification revision was applied")
	}
	if _, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, 0, 1, "worktree", snapshot.Source.Digest, proposal); !errors.Is(err, ErrAnalysisChanged) {
		t.Fatal("stale brief revision was applied")
	}
	saved, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, 0, 0, "worktree", snapshot.Source.Digest, proposal)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Status != "draft" || saved.Digest != "" || saved.Revision != 1 || saved.BriefRevision != 1 || saved.Requirements[0].ID != "AC-delivery" {
		t.Fatal("analysis approved or lost stable requirements")
	}
	data, err := os.ReadFile(filepath.Join(p.Path, ".luci", "specification.md"))
	if err != nil || !strings.Contains(string(data), "AC-delivery") {
		t.Fatal("agent context not exported", err)
	}
	if _, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, 0, 0, "worktree", snapshot.Source.Digest, proposal); !errors.Is(err, ErrAnalysisChanged) {
		t.Fatal("old analysis was applied twice")
	}
}
