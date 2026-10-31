package product

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const githubPublicationBytes = 256 << 10
const githubPublicationRecent = 64

// The reviewed copy is immutable. Mutable provider observations never replace
// its actor, source, CLI, artifact, tag, commit or publication intent.
type GitHubReleaseIntent struct {
	ProjectID            string               `json:"projectId"`
	Repository           string               `json:"repository"`
	RepositoryID         int64                `json:"repositoryId"`
	Tag                  string               `json:"tag"`
	Commit               string               `json:"commit"`
	GitReviewDigest      string               `json:"gitReviewDigest"`
	ContextDigest        string               `json:"contextDigest"`
	ArtifactID           string               `json:"artifactId"`
	ArtifactRevision     int                  `json:"artifactRevision"`
	ArtifactDigest       string               `json:"artifactDigest"`
	ArtifactSourceDigest string               `json:"artifactSourceDigest"`
	Title                string               `json:"title"`
	Body                 string               `json:"body"`
	Account              GitHubReleaseAccount `json:"account"`
}
type GitHubPublication struct {
	ID                  string               `json:"id"`
	ProjectID           string               `json:"projectId"`
	Revision            int                  `json:"revision"`
	Intent              GitHubReleaseIntent  `json:"intent"`
	IntentDigest        string               `json:"intentDigest"`
	State               string               `json:"state"`
	CreatedAt           string               `json:"createdAt"`
	UpdatedAt           string               `json:"updatedAt"`
	ExpiresAt           string               `json:"expiresAt"`
	DraftAttempted      bool                 `json:"draftAttempted"`
	PublishAttempted    bool                 `json:"publishAttempted"`
	Remote              *GitHubReleaseRemote `json:"remote"`
	UnconfirmedRemote   *GitHubReleaseRemote `json:"unconfirmedRemote,omitempty"`
	PublishReviewDigest string               `json:"publishReviewDigest,omitempty"`
	PublishExpiresAt    string               `json:"publishExpiresAt,omitempty"`
	Message             string               `json:"message"`
	CLIAuthScope        string               `json:"cliAuthScope"`
}
type storedGitHubPublication struct {
	GitHubPublication
	Version      int                          `json:"version"`
	AccountScope string                       `json:"accountScope"`
	ProjectScope string                       `json:"projectScope"`
	Actor        management.ExecutionActorRef `json:"actor"`
	CLI          GitHubIssueCLIIdentity       `json:"cli"`
}
type githubPublicationIndex struct {
	Version          int                          `json:"version"`
	ProjectID        string                       `json:"projectId"`
	AccountScope     string                       `json:"accountScope"`
	ProjectScope     string                       `json:"projectScope"`
	Actor            management.ExecutionActorRef `json:"actor"`
	IDs              []string                     `json:"ids"`
	EarlierAvailable bool                         `json:"earlierAvailable"`
}

func validGitHubPublicationID(id string) bool {
	return strings.HasPrefix(id, "gh-release-") && len(id) <= 80 && prototypeReleaseID.MatchString(id)
}
func validGitHubReleaseIntent(intent GitHubReleaseIntent) bool {
	return coolifyID.MatchString(intent.ProjectID) && validFeedbackRepository(intent.Repository) && intent.RepositoryID > 0 && intent.RepositoryID <= 9007199254740991 && githubReleaseTag.MatchString(intent.Tag) && githubReleaseSHA.MatchString(intent.Commit) && hexDigest(intent.GitReviewDigest) && hexDigest(intent.ContextDigest) && prototypeReleaseID.MatchString(intent.ArtifactID) && intent.ArtifactRevision > 0 && hexDigest(intent.ArtifactDigest) && hexDigest(intent.ArtifactSourceDigest) && strings.TrimSpace(intent.Title) != "" && len(intent.Title) <= 200 && strings.TrimSpace(intent.Body) != "" && len(intent.Body) <= 32<<10 && validArtifactStoredText(intent.Title) && validArtifactStoredText(intent.Body) && intent.Account.ID > 0 && intent.Account.ID <= 9007199254740991 && githubReleaseLogin.MatchString(intent.Account.Login)
}
func validGitHubPublication(row storedGitHubPublication) bool {
	if row.Version != 1 || !validGitHubPublicationID(row.ID) || row.ProjectID != row.Intent.ProjectID || !validGitHubReleaseIntent(row.Intent) || row.IntentDigest != reviewDigest(row.Intent) || row.Actor.Local || row.Actor.UserID == "" || row.Actor.ProjectID != row.ProjectID || row.Revision < 1 || row.Revision > 1_000_000_000 || row.CLI.Bytes < 1 || row.CLI.Path == "" || !hexDigest(row.CLI.SHA256) || row.CLIAuthScope != "workspace-cli-not-personal-identity" || len(row.Message) > 2000 || !validArtifactStoredText(row.Message) || !feedbackTimestamp(row.CreatedAt) || !feedbackTimestamp(row.UpdatedAt) || !feedbackTimestamp(row.ExpiresAt) {
		return false
	}
	created, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
	expires, _ := time.Parse(time.RFC3339Nano, row.ExpiresAt)
	if !expires.Equal(created.Add(10 * time.Minute)) {
		return false
	}
	switch row.State {
	case "reviewed", "draft-unknown", "draft", "draft-reviewed", "publish-unknown", "published", "conflict":
	default:
		return false
	}
	if row.Remote != nil && (!validGitHubReleaseRemote(*row.Remote, row.Intent.Repository, row.Intent.Tag) || row.Remote.Author.ID != row.Intent.Account.ID) {
		return false
	}
	if row.UnconfirmedRemote != nil && !validGitHubReleaseRemote(*row.UnconfirmedRemote, row.Intent.Repository, row.Intent.Tag) {
		return false
	}
	if row.PublishAttempted && (!row.DraftAttempted || row.Remote == nil) {
		return false
	}
	if row.State == "reviewed" && (row.DraftAttempted || row.PublishAttempted || row.Remote != nil) {
		return false
	}
	if (row.State == "draft" || row.State == "draft-reviewed") && (!row.DraftAttempted || row.Remote == nil || !row.Remote.Draft) {
		return false
	}
	if row.State == "published" && (!row.PublishAttempted || row.Remote == nil || row.Remote.Draft || row.Remote.PublishedAt == nil) {
		return false
	}
	if row.PublishReviewDigest != "" && (!hexDigest(row.PublishReviewDigest) || !feedbackTimestamp(row.PublishExpiresAt)) {
		return false
	}
	return true
}
func (s *Server) githubPublicationPath(p project.Project, id string) (string, error) {
	if !validGitHubPublicationID(id) {
		return "", errGitHubRelease
	}
	return privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "github-release-"+p.ID+"-"+id)
}

// Caller holds the normal project read lock and githubReleasesMu. Complete
// original attempt files remain addressable after leaving the recent index.
func (s *Server) readGitHubPublication(p project.Project, actor management.ExecutionActorRef, id string) (storedGitHubPublication, error) {
	var row storedGitHubPublication
	path, err := s.githubPublicationPath(p, id)
	if err != nil {
		return row, err
	}
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return row, err
	}
	if externalReadJSON(path, githubPublicationBytes, &row) != nil || !validGitHubPublication(row) || row.Actor != actor || row.ProjectID != p.ID || row.AccountScope != scope.AccountScope || row.ProjectScope != scope.ProjectScope {
		return row, errGitHubRelease
	}
	return row, nil
}
func (s *Server) writeGitHubPublication(p project.Project, row storedGitHubPublication) error {
	if !validGitHubPublication(row) {
		return errGitHubRelease
	}
	path, err := s.githubPublicationPath(p, row.ID)
	if err != nil {
		return err
	}
	raw, err := json.MarshalIndent(row, "", "  ")
	if err != nil || len(raw)+1 > githubPublicationBytes {
		return errGitHubRelease
	}
	if row.Revision == 1 {
		// Exclusive original ownership. A partial initial record is retained and
		// never adopted or overwritten as a different prospective publication.
		file, openErr := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if openErr != nil {
			return openErr
		}
		err = restrictPrivateFile(path)
		if err == nil {
			var count int
			count, err = file.Write(append(raw, '\n'))
			if err == nil && count != len(raw)+1 {
				err = io.ErrShortWrite
			}
		}
		if err == nil {
			err = file.Sync()
		}
		closed := file.Close()
		if err == nil {
			err = closed
		}
		if err == nil {
			err = syncProjectRunDirectory(filepath.Dir(path))
		}
		if err != nil {
			return err
		}
	} else {
		prior, readErr := s.readGitHubPublication(p, row.Actor, row.ID)
		if readErr != nil || prior.Revision+1 != row.Revision || prior.IntentDigest != row.IntentDigest || prior.CLI != row.CLI || prior.CreatedAt != row.CreatedAt || prior.ExpiresAt != row.ExpiresAt || prior.DraftAttempted && !row.DraftAttempted || prior.PublishAttempted && !row.PublishAttempted || prior.Remote != nil && (row.Remote == nil || row.Remote.ID != prior.Remote.ID) {
			return errGitHubRelease
		}
		if err = durableRunJSON(path, row); err != nil {
			return err
		}
	}
	actual, err := s.readGitHubPublication(p, row.Actor, row.ID)
	if err != nil || reviewDigest(actual) != reviewDigest(row) {
		return errGitHubRelease
	}
	return nil
}
func (s *Server) readGitHubPublicationIndex(p project.Project, actor management.ExecutionActorRef) (string, githubPublicationIndex, error) {
	scope, err := s.approvalScopes(p.Path, p.ID)
	if err != nil {
		return "", githubPublicationIndex{}, err
	}
	empty := githubPublicationIndex{Version: 1, ProjectID: p.ID, AccountScope: scope.AccountScope, ProjectScope: scope.ProjectScope, Actor: actor, IDs: []string{}}
	path, err := privateApplicationStatePathForRoot(s.config.ProjectsDir, s.stateDir, "github-release-index-"+p.ID+"-"+reviewDigest(actor)[:24])
	if err != nil {
		return "", empty, err
	}
	var index githubPublicationIndex
	err = externalReadJSON(path, 64<<10, &index)
	if errors.Is(err, os.ErrNotExist) {
		return path, empty, nil
	}
	if err != nil || index.Version != 1 || index.ProjectID != p.ID || index.Actor != actor || index.AccountScope != scope.AccountScope || index.ProjectScope != scope.ProjectScope || index.IDs == nil || len(index.IDs) > githubPublicationRecent {
		return "", empty, errGitHubRelease
	}
	seen := map[string]bool{}
	for _, id := range index.IDs {
		if !validGitHubPublicationID(id) || seen[id] {
			return "", empty, errGitHubRelease
		}
		seen[id] = true
	}
	return path, index, nil
}
func (s *Server) indexGitHubPublication(p project.Project, row storedGitHubPublication) error {
	path, index, err := s.readGitHubPublicationIndex(p, row.Actor)
	if err != nil {
		return err
	}
	for _, id := range index.IDs {
		if id == row.ID {
			return nil
		}
	}
	if len(index.IDs) >= githubPublicationRecent {
		removed := false
		for i := len(index.IDs) - 1; i >= 0; i-- {
			prior, err := s.readGitHubPublication(p, row.Actor, index.IDs[i])
			if err != nil {
				return err
			}
			expired, _ := time.Parse(time.RFC3339Nano, prior.ExpiresAt)
			if prior.State == "published" || !prior.DraftAttempted && time.Now().After(expired) {
				index.IDs = append(index.IDs[:i], index.IDs[i+1:]...)
				index.EarlierAvailable = true
				removed = true
				break
			}
		}
		if !removed {
			return errors.New("64 unresolved GitHub publication records are retained. Observe the original attempts before creating another; none was discarded.")
		}
	}
	index.IDs = append([]string{row.ID}, index.IDs...)
	return durableRunJSON(path, index)
}
func (s *Server) SetGitHubReleaseRunner(runner GitHubReleaseRunner) { s.githubReleaseRunner = runner }
func (s *Server) releaseRunner() GitHubReleaseRunner {
	if s.githubReleaseRunner != nil {
		return s.githubReleaseRunner
	}
	return nativeGitHubReleaseRunner{identity: nativeGitHubIssuesRunner{projectsRoot: s.projectRoot}}
}

// No mutation is issued by these read helpers; caller still needs current read.
func (s *Server) GitHubPublication(ctx context.Context, actor management.ExecutionActorRef, id string) (GitHubPublication, error) {
	if s.feedbackAccess(ctx, actor, false) != nil {
		return GitHubPublication{}, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return GitHubPublication{}, err
	}
	s.mu.RLock()
	s.githubReleasesMu.Lock()
	row, err := s.readGitHubPublication(p, actor, id)
	s.githubReleasesMu.Unlock()
	s.mu.RUnlock()
	if err != nil {
		return GitHubPublication{}, err
	}
	if s.feedbackAccess(ctx, actor, false) != nil {
		return GitHubPublication{}, management.ErrExecutionAuthority
	}
	return row.GitHubPublication, nil
}

// Caller holds githubReleasesMu. Unknown/conflicting originals never leave the
// recent index, so a new preparation cannot bypass their no-replay contract.
func (s *Server) unresolvedGitHubPublication(p project.Project, actor management.ExecutionActorRef, intent GitHubReleaseIntent, except string) error {
	_, index, err := s.readGitHubPublicationIndex(p, actor)
	if err != nil {
		return err
	}
	for _, id := range index.IDs {
		if id == except {
			continue
		}
		prior, err := s.readGitHubPublication(p, actor, id)
		if err != nil {
			return err
		}
		if prior.Intent.RepositoryID == intent.RepositoryID && prior.Intent.Tag == intent.Tag && prior.DraftAttempted && prior.State != "published" {
			return githubPublicationNotice("An original release attempt for this repository and tag is unresolved. Read and observe that original record; a new preparation cannot repeat its effect.")
		}
	}
	return nil
}

// This is one locked local admission: check all retained original attempts and
// publish the attempt marker before a provider mutation can be issued.
func (s *Server) beginGitHubPublicationDraft(p project.Project, row *storedGitHubPublication) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.githubReleasesMu.Lock()
	defer s.githubReleasesMu.Unlock()
	actual, err := s.readGitHubPublication(p, row.Actor, row.ID)
	if err != nil || actual.Revision != row.Revision || actual.IntentDigest != row.IntentDigest || actual.DraftAttempted || actual.PublishAttempted {
		return errGitHubRelease
	}
	if err = s.unresolvedGitHubPublication(p, row.Actor, row.Intent, row.ID); err != nil {
		return err
	}
	row.Revision++
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.writeGitHubPublication(p, *row)
}
