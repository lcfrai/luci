package product

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func publicationFixture(t *testing.T) (*Server, project.Project, storedGitHubPublication) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "GitHub publication")
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	hash := strings.Repeat("a", 64)
	intent := GitHubReleaseIntent{ProjectID: p.ID, Repository: "owner/repo", RepositoryID: 7, Tag: "v1.0.0", Commit: strings.Repeat("b", 40), GitReviewDigest: hash, ContextDigest: hash, ArtifactID: "artifact-launch", ArtifactRevision: 1, ArtifactDigest: hash, ArtifactSourceDigest: hash, Title: "A reviewed launch", Body: "Full public copy\n", Account: GitHubReleaseAccount{ID: 11, Login: "publisher"}}
	row := storedGitHubPublication{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: management.ExecutionActorRef{UserID: "alice", ProjectID: p.ID}, CLI: GitHubIssueCLIIdentity{Path: "E:/tools/gh.exe", Bytes: 1, SHA256: hash}, GitHubPublication: GitHubPublication{ID: newID("gh-release-"), ProjectID: p.ID, Revision: 1, Intent: intent, IntentDigest: reviewDigest(intent), State: "reviewed", CreatedAt: at.Format(time.RFC3339Nano), UpdatedAt: at.Format(time.RFC3339Nano), ExpiresAt: at.Add(10 * time.Minute).Format(time.RFC3339Nano), Message: "Public copy reviewed; no GitHub effect.", CLIAuthScope: "workspace-cli-not-personal-identity"}}
	if err = s.writeGitHubPublication(p, row); err != nil {
		t.Fatal(err)
	}
	if err = s.indexGitHubPublication(p, row); err != nil {
		t.Fatal(err)
	}
	return s, p, row
}

func TestGitHubPublicationOriginalAttemptBlocksAlternateIntent(t *testing.T) {
	s, p, original := publicationFixture(t)
	alternate := original
	alternate.ID = newID("gh-release-")
	if err := s.writeGitHubPublication(p, alternate); err != nil {
		t.Fatal(err)
	}
	if err := s.indexGitHubPublication(p, alternate); err != nil {
		t.Fatal(err)
	}
	original.DraftAttempted = true
	original.State = "draft-unknown"
	if err := s.beginGitHubPublicationDraft(p, &original); err != nil {
		t.Fatal(err)
	}
	if err := s.unresolvedGitHubPublication(p, original.Actor, original.Intent, ""); err == nil {
		t.Fatal("a new preparation bypassed the original unknown attempt")
	}
	alternate.DraftAttempted = true
	alternate.State = "draft-unknown"
	if err := s.beginGitHubPublicationDraft(p, &alternate); err == nil {
		t.Fatal("an already reviewed alternate ID repeated the same tag effect")
	}
	saved, err := s.readGitHubPublication(p, alternate.Actor, alternate.ID)
	if err != nil || saved.DraftAttempted || saved.Revision != 1 {
		t.Fatal("refused alternate admission mutated its journal", err)
	}
	if err = s.beginGitHubPublicationDraft(p, &original); err == nil {
		t.Fatal("original draft attempt replayed")
	}
	if _, err = s.readGitHubPublication(p, management.ExecutionActorRef{UserID: "other", ProjectID: p.ID}, original.ID); err == nil {
		t.Fatal("actor-private original leaked")
	}
}

func TestGitHubPublicationRetainsCorrelatedUnexpectedCopyWithoutAdoption(t *testing.T) {
	s, p, row := publicationFixture(t)
	row.DraftAttempted = true
	row.State = "draft-unknown"
	if err := s.beginGitHubPublicationDraft(p, &row); err != nil {
		t.Fatal(err)
	}
	remote := GitHubReleaseRemote{ID: 42, Tag: row.Intent.Tag, Commitish: row.Intent.Commit, Title: row.Intent.Title, Body: "Different remote copy", Draft: true, URL: "https://github.com/Owner/Repo/releases/tag/v1.0.0", CreatedAt: row.CreatedAt, Author: row.Intent.Account}
	if !githubRemoteCorrelated(&remote, row.Intent) || githubRemoteMatches(&remote, row.Intent) {
		t.Fatal("correlation confused with reviewed content")
	}
	if err := s.settleGitHubPublication(p, &row, &remote, "conflict", "Original response retained; content differs."); err != nil {
		t.Fatal(err)
	}
	actual, err := s.readGitHubPublication(p, row.Actor, row.ID)
	if err != nil || actual.Remote == nil || actual.Remote.ID != 42 || actual.State != "conflict" {
		t.Fatal("positive original ID lost", err)
	}
	remote.ID = 43
	if err = s.settleGitHubPublication(p, &row, &remote, "draft", "replacement"); err == nil {
		t.Fatal("another remote ID adopted")
	}
	if _, err = s.ActGitHubPublication(context.Background(), row.Actor, row.ID, "publish", GitHubPublicationAction{ExpectedRevision: row.Revision, IntentDigest: row.IntentDigest, AcknowledgeContent: true}); err == nil {
		t.Fatal("missing fresh deployment authority allowed publication")
	}
}

func TestGitHubPublicationFullCopyBoundAndClosedPayload(t *testing.T) {
	artifact := DistributeArtifact{Sections: []DistributeArtifactSection{{Heading: "Launch", Body: "Complete reviewed copy"}}, Assumptions: []string{"Demand remains a hypothesis"}}
	body, err := githubReleaseBody(artifact)
	if err != nil || !strings.Contains(body, "Demand remains a hypothesis") {
		t.Fatal("assumptions dropped", err)
	}
	artifact.Sections[0].Body = strings.Repeat("x", 33<<10)
	if _, err = githubReleaseBody(artifact); err == nil {
		t.Fatal("oversized full copy silently truncated")
	}
	_, _, row := publicationFixture(t)
	payload := githubReleasePayload(row.Intent, true)
	if payload["draft"] != true || payload["make_latest"] != "false" || payload["target_commitish"] != row.Intent.Commit || payload["body"] != row.Intent.Body {
		t.Fatal("payload changed reviewed intent")
	}
	for _, tag := range []string{"../bad", "refs/tags/v1", "-flag", "v1?query"} {
		if githubReleaseTag.MatchString(tag) {
			t.Fatalf("unsafe tag %q", tag)
		}
	}
}

type publicationTagFixture struct {
	GitHubReleaseRunner
	intent   GitHubReleaseIntent
	commit   string
	tagReads int
}

func (f *publicationTagFixture) Account(context.Context, GitHubIssueCLIIdentity) (GitHubReleaseAccount, error) {
	return f.intent.Account, nil
}
func (f *publicationTagFixture) RepositoryID(context.Context, GitHubIssueCLIIdentity, string) (int64, error) {
	return f.intent.RepositoryID, nil
}
func (f *publicationTagFixture) TagCommit(context.Context, GitHubIssueCLIIdentity, string, string) (string, error) {
	f.tagReads++
	return f.commit, nil
}

func TestGitHubPublicationBranchMetadataStillRequiresExactResolvedTag(t *testing.T) {
	s, _, row := publicationFixture(t)
	remote := GitHubReleaseRemote{ID: 42, Tag: row.Intent.Tag, Commitish: "main", Title: row.Intent.Title, Body: row.Intent.Body, Draft: true, URL: "https://github.com/owner/repo/releases/tag/untagged-original", CreatedAt: row.CreatedAt, Author: row.Intent.Account}
	if !githubRemoteMatches(&remote, row.Intent) {
		t.Fatal("branch display metadata incorrectly rejected an exact existing-tag release")
	}
	provider := &publicationTagFixture{intent: row.Intent, commit: row.Intent.Commit}
	if err := s.checkGitHubReleaseProvider(context.Background(), provider, row); err != nil || provider.tagReads != 1 {
		t.Fatal("exact resolved tag was not required", err)
	}
	provider.commit = strings.Repeat("d", 40)
	if err := s.checkGitHubReleaseProvider(context.Background(), provider, row); err == nil || provider.tagReads != 2 {
		t.Fatal("changed resolved tag was accepted because display metadata matched")
	}
}
