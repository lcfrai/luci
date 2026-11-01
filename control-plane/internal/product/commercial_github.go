package product

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const githubCommercialSemantics = "GitHub counters for assets currently returned by this original published release, plus reactions when provided. Individual pages are observed during collection, not an atomic snapshot. Removed assets are absent. Downloads are not unique people, release views, conversions or revenue. Authentication is the workspace GitHub CLI login, not proof of the Luci person's GitHub identity."

func fmtCommercialID(id int64) string { return strconv.FormatInt(id, 10) }

type commercialGitHubAsset struct {
	ID        int64  `json:"id"`
	Downloads *int64 `json:"download_count"`
}
type commercialGitHubPage struct {
	Assets    []commercialGitHubAsset
	Reactions *int64
	More      bool
}
type commercialGitHubReader interface {
	Identity(context.Context) (GitHubIssueCLIIdentity, error)
	RepositoryID(context.Context, GitHubIssueCLIIdentity, string) (int64, error)
	TagCommit(context.Context, GitHubIssueCLIIdentity, string, string) (string, error)
	CommercialPage(context.Context, GitHubIssueCLIIdentity, CommercialReleaseSource, int) (commercialGitHubPage, error)
}

func (s *Server) commercialGitHubClient() commercialGitHubReader {
	if s.commercialGitHub != nil {
		return s.commercialGitHub
	}
	runner, ok := s.releaseRunner().(commercialGitHubReader)
	if !ok {
		return nil
	}
	return runner
}
func (n nativeGitHubReleaseRunner) CommercialPage(ctx context.Context, cli GitHubIssueCLIIdentity, source CommercialReleaseSource, page int) (commercialGitHubPage, error) {
	if !validFeedbackRepository(source.Repository) || source.ReleaseID < 1 || source.ReleaseID > commercialSafeInteger || page < 1 || page > 1001 {
		return commercialGitHubPage{}, ErrCommercial
	}
	endpoint := "repos/" + source.Repository + "/releases/" + fmtCommercialID(source.ReleaseID)
	status, raw, err := n.request(ctx, cli, "GET", endpoint, nil)
	var metadata struct {
		ID        int64   `json:"id"`
		Tag       string  `json:"tag_name"`
		Draft     *bool   `json:"draft"`
		Published *string `json:"published_at"`
		Author    struct {
			ID int64 `json:"id"`
		} `json:"author"`
		Reactions *struct {
			Total *int64 `json:"total_count"`
		} `json:"reactions"`
	}
	if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &metadata) != nil || metadata.ID != source.ReleaseID || metadata.Tag != source.Tag || metadata.Draft == nil || *metadata.Draft || metadata.Published == nil || !feedbackTimestamp(*metadata.Published) || metadata.Author.ID != source.AuthorID {
		return commercialGitHubPage{}, ErrCommercial
	}
	result := commercialGitHubPage{}
	if metadata.Reactions != nil {
		if metadata.Reactions.Total == nil || *metadata.Reactions.Total < 0 || *metadata.Reactions.Total > commercialSafeInteger {
			return result, ErrCommercial
		}
		result.Reactions = metadata.Reactions.Total
	}
	status, raw, err = n.request(ctx, cli, "GET", endpoint+"/assets?per_page=100&page="+strconv.Itoa(page), nil)
	if err != nil || status != 200 || json.Unmarshal(raw, &result.Assets) != nil || result.Assets == nil || len(result.Assets) > 100 {
		return commercialGitHubPage{}, ErrCommercial
	}
	for _, asset := range result.Assets {
		if asset.ID < 1 || asset.ID > commercialSafeInteger || asset.Downloads == nil || *asset.Downloads < 0 || *asset.Downloads > commercialSafeInteger {
			return commercialGitHubPage{}, ErrCommercial
		}
	}
	result.More = len(result.Assets) == 100
	return result, nil
}
func collectGitHubCommercialPage(collection *commercialCollection, page commercialGitHubPage) error {
	if page.Assets == nil || len(page.Assets) > 100 || page.More && len(page.Assets) == 0 {
		return ErrCommercial
	}
	seen := commercialSeen(collection.SeenRecords)
	newRecords := 0
	for _, asset := range page.Assets {
		if asset.ID < 1 || asset.ID > commercialSafeInteger || asset.Downloads == nil || *asset.Downloads < 0 || *asset.Downloads > commercialSafeInteger {
			return ErrCommercial
		}
		key := commercialDedup(collection, "asset", fmtCommercialID(asset.ID))
		if seen[key] {
			continue
		}
		if collection.Snapshot.Records >= commercialRecordLimit {
			return errCommercialWindowLimit
		}
		seen[key] = true
		collection.SeenRecords = append(collection.SeenRecords, key)
		collection.Snapshot.Records++
		newRecords++
		var err error
		collection.Snapshot.AssetDownloads, err = commercialAdd(collection.Snapshot.AssetDownloads, *asset.Downloads)
		if err != nil {
			return err
		}
	}
	if page.More && newRecords == 0 {
		return ErrCommercial
	}
	collection.Page++
	collection.Snapshot.Reactions = page.Reactions
	collection.Complete = !page.More
	return nil
}

type CommercialReleaseChoice struct {
	PublicationID string `json:"publicationId"`
	Label         string `json:"label"`
	Repository    string `json:"repository"`
	Tag           string `json:"tag"`
}

func (s *Server) CommercialReleaseChoices(ctx context.Context, actor management.ExecutionActorRef) ([]CommercialReleaseChoice, error) {
	if s.feedbackAccess(ctx, actor, false) != nil {
		return nil, management.ErrExecutionAuthority
	}
	p, err := s.resolveProject(actor.ProjectID)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	s.githubReleasesMu.Lock()
	_, index, err := s.readGitHubPublicationIndex(p, actor)
	out := []CommercialReleaseChoice{}
	if err == nil {
		for _, id := range index.IDs {
			row, e := s.readGitHubPublication(p, actor, id)
			if e != nil {
				err = e
				break
			}
			if row.State == "published" && row.Remote != nil && !row.Remote.Draft {
				out = append(out, CommercialReleaseChoice{row.ID, row.Intent.Title, row.Intent.Repository, row.Intent.Tag})
			}
		}
	}
	s.githubReleasesMu.Unlock()
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if s.feedbackAccess(ctx, actor, false) != nil {
		return nil, management.ErrExecutionAuthority
	}
	return out, nil
}
func (s *Server) commercialReleaseBinding(p project.Project, actor management.ExecutionActorRef, id string) (CommercialReleaseSource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.githubReleasesMu.Lock()
	defer s.githubReleasesMu.Unlock()
	row, err := s.readGitHubPublication(p, actor, id)
	if err != nil || row.State != "published" || row.Remote == nil || row.Remote.Draft {
		return CommercialReleaseSource{}, ErrCommercial
	}
	return CommercialReleaseSource{PublicationID: row.ID, Repository: row.Intent.Repository, RepositoryID: row.Intent.RepositoryID, ReleaseID: row.Remote.ID, AuthorID: row.Intent.Account.ID, Tag: row.Intent.Tag, Commit: row.Intent.Commit, ArtifactID: row.Intent.ArtifactID, ArtifactDigest: row.Intent.ArtifactDigest, Label: row.Intent.Title}, nil
}
