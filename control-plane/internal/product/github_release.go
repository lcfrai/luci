package product

import (
	"context"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

type githubPublicationNotice string

func (e githubPublicationNotice) Error() string { return string(e) }

type GitHubPublicationReview struct {
	ArtifactID       string `json:"artifactId"`
	ArtifactRevision int    `json:"artifactRevision"`
	ArtifactDigest   string `json:"artifactDigest"`
	Tag              string `json:"tag"`
}
type GitHubPublicationAction struct {
	ExpectedRevision   int    `json:"expectedRevision"`
	IntentDigest       string `json:"intentDigest"`
	AcknowledgeContent bool   `json:"acknowledgeContent"`
}

// No Execute or new permission exists here. Current deployment authority plus
// its CURRENT intersected Secrets grant and public-publishing policy govern
// new release effects. Reading retained local history needs only project Read.
func (s *Server) githubReleaseAuthority(ctx context.Context, actor management.ExecutionActorRef, publishing bool) (context.Context, error) {
	if ctx.Err() != nil || actor.Local || actor.UserID == "" || s.coordinatorDeploymentAuthorize == nil {
		return ctx, management.ErrExecutionAuthority
	}
	authority, err := s.coordinatorDeploymentAuthorize(ctx, actor, publishing)
	if err != nil || authority.Actor.ID != actor.UserID || authority.Actor.Disabled || publishing && !authority.Policy.AllowPublicPublishing {
		return ctx, management.ErrExecutionAuthority
	}
	if actor.TeamID != "" && (authority.Team == nil || authority.Team.Team.ID != actor.TeamID || authority.Team.ProjectID != actor.ProjectID || !authority.Team.Permissions.Read || !authority.Team.Permissions.Edit || !authority.Team.Permissions.Deploy || !authority.Team.Permissions.Secrets) {
		return ctx, management.ErrExecutionAuthority
	}
	current := management.WithExecutionAuthority(ctx, authority)
	actual, err := management.CaptureDeploymentActor(current, actor.ProjectID)
	if err != nil || actual != actor {
		return ctx, management.ErrExecutionAuthority
	}
	return current, current.Err()
}
func githubReleaseBody(artifact DistributeArtifact) (string, error) {
	var out strings.Builder
	for _, section := range artifact.Sections {
		out.WriteString("## " + section.Heading + "\n\n")
		if section.Body != "" {
			out.WriteString(section.Body + "\n\n")
		}
		for _, bullet := range section.Bullets {
			out.WriteString("- " + bullet + "\n")
		}
		out.WriteString("\n")
	}
	if len(artifact.Assumptions) > 0 {
		out.WriteString("## Assumptions\n\n")
		for _, value := range artifact.Assumptions {
			out.WriteString("- " + value + "\n")
		}
	}
	if len(artifact.Gaps) > 0 {
		out.WriteString("\n## Open questions and limitations\n\n")
		for _, value := range artifact.Gaps {
			out.WriteString("- " + value + "\n")
		}
	}
	body := strings.TrimSpace(out.String()) + "\n"
	if len(body) > 32<<10 || strings.TrimSpace(body) == "" || !validArtifactStoredText(body) {
		return "", githubPublicationNotice("This launch asset exceeds the 32 KiB full-review limit. Edit and review a shorter asset in the existing launch workspace before publishing.")
	}
	return body, nil
}

// Caller holds s.mu followed by teamControlsMu. This reuses normal artifact
// source calculation, including exact GTM revision, not a parallel source map.
func (s *Server) githubReleaseSourceLocked(ctx context.Context, p project.Project, id string) (GitHubReleaseIntent, error) {
	var intent GitHubReleaseIntent
	_, ledger, err := s.readArtifactLedger(p)
	if err != nil {
		return intent, err
	}
	saved := artifactRecord(&ledger, id)
	if saved == nil || len(saved.Revisions) == 0 {
		return intent, errGitHubRelease
	}
	artifact := saved.Revisions[len(saved.Revisions)-1]
	if !launchArtifactKind(artifact.Kind) || artifact.State != "reviewed" || artifact.Review == nil {
		return intent, githubPublicationNotice("Choose an explicitly reviewed launch asset. Saving a draft alone does not review its public content.")
	}
	selectedRun := ""
	if artifact.Source.SelectedEvidence != nil {
		selectedRun = artifact.Source.SelectedEvidence.RunID
	}
	source, _, _, _, err := s.artifactSourceForKindLocked(ctx, p, artifact.Kind, selectedRun)
	if err != nil || artifact.SourceDigest != reviewDigest(source) {
		return intent, errGitHubRelease
	}
	feedback, err := s.feedbackSourceLocked(ctx, p)
	if err != nil {
		return intent, err
	}
	bounded, cancel := context.WithTimeout(context.WithValue(ctx, journeyGitRootContextKey{}, s.config.ProjectsDir), 3*time.Second)
	defer cancel()
	git, err := s.captureGitReview(bounded, p.Path, "")
	if err != nil || git.Status != "" || !githubReleaseSHA.MatchString(git.Head) || !hexDigest(git.Digest) {
		return intent, githubPublicationNotice("Commit the current project source before reviewing its existing GitHub tag. No Git push is performed here.")
	}
	body, err := githubReleaseBody(artifact)
	if err != nil {
		return intent, err
	}
	return GitHubReleaseIntent{ProjectID: p.ID, Repository: feedback.Repository, Commit: git.Head, GitReviewDigest: git.Digest, ContextDigest: feedback.ContextDigest, ArtifactID: artifact.ID, ArtifactRevision: artifact.Revision, ArtifactDigest: artifact.Digest, ArtifactSourceDigest: artifact.SourceDigest, Title: artifact.Title, Body: body}, nil
}
func sameGitHubReleaseSource(a, b GitHubReleaseIntent) bool {
	// Tag/account/repository numeric identity are separately resolved provider
	// facts. Everything below is the current exact project and artifact snapshot.
	a.RepositoryID = 0
	b.RepositoryID = 0
	a.Tag = ""
	b.Tag = ""
	a.Account = GitHubReleaseAccount{}
	b.Account = GitHubReleaseAccount{}
	return a == b
}
func (s *Server) githubReleaseGuard(ctx context.Context, p project.Project, actor management.ExecutionActorRef, intent GitHubReleaseIntent, publishing, currentSource bool) error {
	current, err := s.githubReleaseAuthority(ctx, actor, publishing)
	if err != nil {
		return err
	}
	actual, err := s.resolveProject(p.ID)
	if err != nil || actual.Path != p.Path {
		return errGitHubRelease
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if currentSource {
		s.teamControlsMu.Lock()
		defer s.teamControlsMu.Unlock()
		captured, err := s.githubReleaseSourceLocked(current, p, intent.ArtifactID)
		if err != nil {
			return err
		}
		if !sameGitHubReleaseSource(captured, intent) {
			return errGitHubRelease
		}
	} else {
		// Retained remote observations may inspect historical content, but never
		// use credentials against a repository detached from this project.
		bounded, cancel := context.WithTimeout(context.WithValue(current, journeyGitRootContextKey{}, s.config.ProjectsDir), 3*time.Second)
		defer cancel()
		origin, err := s.journeyGit(bounded, p.Path, "config", "--get-all", "remote.origin.url")
		if err != nil {
			return errGitHubRelease
		}
		repo, err := canonicalGitHubOrigin(strings.TrimSpace(origin))
		if err != nil || repo != intent.Repository {
			return errGitHubRelease
		}
	}
	return current.Err()
}
func (s *Server) githubReleaseIO(ctx context.Context, p project.Project, actor management.ExecutionActorRef, intent GitHubReleaseIntent, publishing, currentSource bool) context.Context {
	return context.WithValue(ctx, githubReleaseGuardKey{}, func(current context.Context) error {
		return s.githubReleaseGuard(current, p, actor, intent, publishing, currentSource)
	})
}
func sameGitHubAccount(a, b GitHubReleaseAccount) bool {
	return a.ID == b.ID && strings.EqualFold(a.Login, b.Login)
}
func (s *Server) checkGitHubReleaseProvider(ctx context.Context, runner GitHubReleaseRunner, row storedGitHubPublication) error {
	account, err := runner.Account(ctx, row.CLI)
	if err != nil || !sameGitHubAccount(account, row.Intent.Account) {
		return errGitHubRelease
	}
	repository, err := runner.RepositoryID(ctx, row.CLI, row.Intent.Repository)
	if err != nil || repository != row.Intent.RepositoryID {
		return errGitHubRelease
	}
	commit, err := runner.TagCommit(ctx, row.CLI, row.Intent.Repository, row.Intent.Tag)
	if err != nil || commit != row.Intent.Commit {
		return errGitHubRelease
	}
	return nil
}
func githubRemoteCorrelated(row *GitHubReleaseRemote, intent GitHubReleaseIntent) bool {
	return row != nil && validGitHubReleaseRemote(*row, intent.Repository, intent.Tag) && row.Author.ID == intent.Account.ID
}
func githubRemoteMatches(row *GitHubReleaseRemote, intent GitHubReleaseIntent) bool {
	// target_commitish is provider display metadata (possibly a branch), not
	// immutable tag authority. checkGitHubReleaseProvider separately resolves
	// the exact tag to the reviewed full commit before and after mutations.
	return row != nil && validGitHubReleaseRemote(*row, intent.Repository, intent.Tag) && row.Author.ID == intent.Account.ID && row.Title == intent.Title && row.Body == intent.Body && !row.Prerelease
}
func (s *Server) PrepareGitHubPublication(ctx context.Context, actor management.ExecutionActorRef, input GitHubPublicationReview) (GitHubPublication, error) {
	bounded, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	ctx = bounded
	current, err := s.githubReleaseAuthority(ctx, actor, true)
	if err != nil {
		return GitHubPublication{}, err
	}
	if !githubReleaseTag.MatchString(input.Tag) || input.ArtifactRevision < 1 || !hexDigest(input.ArtifactDigest) {
		return GitHubPublication{}, errGitHubRelease
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return GitHubPublication{}, err
	}
	s.mu.RLock()
	s.teamControlsMu.Lock()
	intent, err := s.githubReleaseSourceLocked(current, p, input.ArtifactID)
	s.teamControlsMu.Unlock()
	s.mu.RUnlock()
	if err != nil {
		return GitHubPublication{}, err
	}
	if intent.ArtifactRevision != input.ArtifactRevision || intent.ArtifactDigest != input.ArtifactDigest {
		return GitHubPublication{}, errGitHubRelease
	}
	intent.Tag = input.Tag
	runner := s.releaseRunner()
	ioctx := s.githubReleaseIO(ctx, p, actor, intent, true, true)
	cli, err := runner.Identity(ioctx)
	if err != nil {
		return GitHubPublication{}, err
	}
	intent.Account, err = runner.Account(ioctx, cli)
	if err != nil {
		return GitHubPublication{}, err
	}
	intent.RepositoryID, err = runner.RepositoryID(ioctx, cli, intent.Repository)
	if err != nil {
		return GitHubPublication{}, err
	}
	commit, err := runner.TagCommit(ioctx, cli, intent.Repository, intent.Tag)
	if err != nil || commit != intent.Commit {
		return GitHubPublication{}, githubPublicationNotice("Create and push this exact tag for the reviewed commit through your existing Git workflow first. Luci does not push tags. GitHub may recreate the verified tag at the reviewed commit if it disappears during draft creation.")
	}
	existing, err := runner.Read(ioctx, cli, intent.Repository, 0, intent.Tag)
	if err != nil {
		return GitHubPublication{}, err
	}
	if existing != nil {
		return GitHubPublication{}, githubPublicationNotice("This tag already has a GitHub release. It was not adopted or overwritten. Choose an unused release tag or manage that release on GitHub.")
	}
	if !validGitHubReleaseIntent(intent) || s.githubReleaseGuard(ctx, p, actor, intent, true, true) != nil {
		return GitHubPublication{}, errGitHubRelease
	}
	now := time.Now().UTC()
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return GitHubPublication{}, err
	}
	row := storedGitHubPublication{Version: 1, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: actor, CLI: cli, GitHubPublication: GitHubPublication{ID: newID("gh-release-"), ProjectID: p.ID, Revision: 1, Intent: intent, IntentDigest: reviewDigest(intent), State: "reviewed", CreatedAt: now.Format(time.RFC3339Nano), UpdatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(10 * time.Minute).Format(time.RFC3339Nano), Message: "Review the complete public copy. No GitHub draft, tag or public release has been created.", CLIAuthScope: "workspace-cli-not-personal-identity"}}
	s.mu.RLock()
	s.githubReleasesMu.Lock()
	err = s.unresolvedGitHubPublication(p, actor, intent, "")
	if err == nil {
		err = s.writeGitHubPublication(p, row)
	}
	if err == nil {
		err = s.indexGitHubPublication(p, row)
	}
	s.githubReleasesMu.Unlock()
	s.mu.RUnlock()
	if err != nil {
		return GitHubPublication{}, err
	}
	if s.githubReleaseGuard(ctx, p, actor, intent, true, true) != nil {
		return GitHubPublication{}, management.ErrExecutionAuthority
	}
	return row.GitHubPublication, nil
}
func (s *Server) persistGitHubPublicationStep(p project.Project, row *storedGitHubPublication) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.githubReleasesMu.Lock()
	defer s.githubReleasesMu.Unlock()
	actual, err := s.readGitHubPublication(p, row.Actor, row.ID)
	if err != nil || actual.Revision != row.Revision || actual.IntentDigest != row.IntentDigest || actual.CLI != row.CLI {
		return errGitHubRelease
	}
	row.Revision++
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.writeGitHubPublication(p, *row)
}

// Settlement is metadata-only for the original attempted operation. A revoked
// caller cannot receive it, but does not cause an already-issued remote ID to
// be discarded or a provider operation to be replayed.
func (s *Server) settleGitHubPublication(p project.Project, row *storedGitHubPublication, remote *GitHubReleaseRemote, state, message string) error {
	if remote != nil {
		if !githubRemoteCorrelated(remote, row.Intent) || row.Remote != nil && row.Remote.ID != remote.ID {
			return errGitHubRelease
		}
		row.Remote = remote
	}
	row.State, row.Message = state, message
	return s.persistGitHubPublicationStep(p, row)
}
func (s *Server) ActGitHubPublication(ctx context.Context, actor management.ExecutionActorRef, id, action string, input GitHubPublicationAction) (GitHubPublication, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	publishing := action != "observe"
	if _, err := s.githubReleaseAuthority(ctx, actor, publishing); err != nil {
		return GitHubPublication{}, err
	}
	if action != "draft" && action != "review" && action != "publish" && action != "observe" {
		return GitHubPublication{}, errGitHubRelease
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return GitHubPublication{}, err
	}
	if !s.reserveDelivery("github-release:" + p.ID + ":" + id) {
		return GitHubPublication{}, githubPublicationNotice("The original release operation is still running. Read its retained record after it settles.")
	}
	defer s.releaseDelivery("github-release:" + p.ID + ":" + id)
	s.mu.RLock()
	s.githubReleasesMu.Lock()
	row, err := s.readGitHubPublication(p, actor, id)
	s.githubReleasesMu.Unlock()
	s.mu.RUnlock()
	if err != nil || row.Revision != input.ExpectedRevision || row.IntentDigest != input.IntentDigest {
		return GitHubPublication{}, errGitHubRelease
	}
	if (action == "draft" || action == "publish") && !input.AcknowledgeContent {
		return GitHubPublication{}, githubPublicationNotice("Review the complete title and body and explicitly confirm this separate GitHub effect.")
	}
	if action == "observe" && !row.DraftAttempted {
		return row.GitHubPublication, nil
	}
	runner := s.releaseRunner()
	ioctx := s.githubReleaseIO(ctx, p, actor, row.Intent, publishing, publishing)
	if err = s.githubReleaseGuard(ioctx, p, actor, row.Intent, publishing, publishing); err != nil {
		return GitHubPublication{}, err
	}
	if err = s.checkGitHubReleaseProvider(ioctx, runner, row); err != nil {
		return GitHubPublication{}, err
	}
	if action == "observe" {
		remoteID := int64(0)
		if row.Remote != nil {
			remoteID = row.Remote.ID
		}
		remote, readErr := runner.Read(ioctx, row.CLI, row.Intent.Repository, remoteID, row.Intent.Tag)
		if readErr != nil {
			return GitHubPublication{}, readErr
		}
		if s.githubReleaseGuard(ctx, p, actor, row.Intent, false, false) != nil {
			return GitHubPublication{}, management.ErrExecutionAuthority
		}
		if remoteID == 0 {
			row.UnconfirmedRemote = remote
			row.Message = "No immutable remote ID was returned by the original draft attempt. A same-tag release is unconfirmed correlation only; it is not adopted, published or created again. Inspect GitHub manually."
			if err = s.persistGitHubPublicationStep(p, &row); err != nil {
				return GitHubPublication{}, err
			}
		} else if remote == nil || !githubRemoteMatches(remote, row.Intent) {
			if err = s.settleGitHubPublication(p, &row, nil, "conflict", "The original remote release changed or is unavailable. No remote copy was overwritten or replayed."); err != nil {
				return GitHubPublication{}, err
			}
		} else {
			state := "draft"
			if !remote.Draft {
				if !row.PublishAttempted {
					return GitHubPublication{}, githubPublicationNotice("This release was published outside this original operation. It is not treated as Luci publication acceptance.")
				}
				state = "published"
			}
			if err = s.settleGitHubPublication(p, &row, remote, state, "Observed the exact original release and existing tag. This is release metadata, not application deployment or behavior verification."); err != nil {
				return GitHubPublication{}, err
			}
		}
	} else {
		expires, _ := time.Parse(time.RFC3339Nano, row.ExpiresAt)
		if action == "draft" {
			if row.DraftAttempted || row.PublishAttempted || row.State != "reviewed" || !time.Now().Before(expires) {
				return GitHubPublication{}, errGitHubRelease
			}
			existing, readErr := runner.Read(ioctx, row.CLI, row.Intent.Repository, 0, row.Intent.Tag)
			if readErr != nil || existing != nil {
				return GitHubPublication{}, errGitHubRelease
			}
			if s.githubReleaseGuard(ctx, p, actor, row.Intent, true, true) != nil {
				return GitHubPublication{}, management.ErrExecutionAuthority
			}
			row.DraftAttempted = true
			row.State = "draft-unknown"
			row.Message = "Original draft creation intent retained. Its outcome is unknown until an exact returned ID is recorded. No automatic retry."
			if err = s.beginGitHubPublicationDraft(p, &row); err != nil {
				return GitHubPublication{}, err
			}
			remote, effectErr := runner.CreateDraft(ioctx, row.CLI, row.Intent)
			if githubRemoteCorrelated(&remote, row.Intent) {
				state := "draft-unknown"
				message := "The original draft ID is retained; late authority/source confirmation is unavailable. Observe after access is restored."
				if effectErr != nil || !githubRemoteMatches(&remote, row.Intent) || !remote.Draft {
					state = "conflict"
					message = "The original returned ID is retained, but its copy or draft state differs from the review. No replacement or publication is permitted."
				} else if s.githubReleaseGuard(ctx, p, actor, row.Intent, true, true) == nil && s.checkGitHubReleaseProvider(ioctx, runner, row) == nil {
					state = "draft"
					message = "GitHub draft created. It is not published. Review the original draft separately before publication."
				}
				if err = s.settleGitHubPublication(p, &row, &remote, state, message); err != nil {
					return GitHubPublication{}, err
				}
			}
		} else {
			if !row.DraftAttempted || row.Remote == nil || row.PublishAttempted {
				return GitHubPublication{}, errGitHubRelease
			}
			remote, readErr := runner.Read(ioctx, row.CLI, row.Intent.Repository, row.Remote.ID, row.Intent.Tag)
			if readErr != nil || !githubRemoteMatches(remote, row.Intent) || !remote.Draft {
				return GitHubPublication{}, errGitHubRelease
			}
			if s.githubReleaseGuard(ctx, p, actor, row.Intent, true, true) != nil {
				return GitHubPublication{}, management.ErrExecutionAuthority
			}
			if action == "review" {
				row.Remote = remote
				row.State = "draft-reviewed"
				row.PublishReviewDigest = reviewDigest(remote)
				row.PublishExpiresAt = time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339Nano)
				row.Message = "The original remote draft and current source were read. Review the complete copy, then explicitly Publish; no publication occurred."
				if err = s.persistGitHubPublicationStep(p, &row); err != nil {
					return GitHubPublication{}, err
				}
			} else {
				publishExpiry, _ := time.Parse(time.RFC3339Nano, row.PublishExpiresAt)
				if row.State != "draft-reviewed" || row.PublishReviewDigest != reviewDigest(remote) || !time.Now().Before(publishExpiry) {
					return GitHubPublication{}, errGitHubRelease
				}
				row.PublishAttempted = true
				row.State = "publish-unknown"
				row.Message = "Original publication intent retained. Its outcome is unknown until the exact ID and copy are observed; no automatic retry."
				if err = s.persistGitHubPublicationStep(p, &row); err != nil {
					return GitHubPublication{}, err
				}
				published, effectErr := runner.Publish(ioctx, row.CLI, row.Intent, row.Remote.ID)
				if githubRemoteCorrelated(&published, row.Intent) && published.ID == row.Remote.ID {
					state := "publish-unknown"
					message := "Original release response retained. Late source/access verification is unavailable; observe it after access is restored."
					if effectErr != nil || !githubRemoteMatches(&published, row.Intent) || published.Draft || published.PublishedAt == nil {
						state = "conflict"
						message = "The original publication ID is retained, but the returned copy or state differs from the review. No mutation is repeated."
					} else if s.githubReleaseGuard(ctx, p, actor, row.Intent, true, true) == nil && s.checkGitHubReleaseProvider(ioctx, runner, row) == nil {
						state = "published"
						message = "The exact original GitHub release was published and observed. It was not made latest. No application deployment, outreach or analytics verification is implied."
					}
					if err = s.settleGitHubPublication(p, &row, &published, state, message); err != nil {
						return GitHubPublication{}, err
					}
				}
			}
		}
	}
	if s.feedbackAccess(ctx, actor, false) != nil {
		return GitHubPublication{}, management.ErrExecutionAuthority
	}
	return row.GitHubPublication, nil
}
