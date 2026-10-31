package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Narrow typed operations only. No HTTP caller supplies an endpoint, method,
// executable, credentials, workflow, attachment or arbitrary CLI argument.
type GitHubReleaseRunner interface {
	Identity(context.Context) (GitHubIssueCLIIdentity, error)
	Account(context.Context, GitHubIssueCLIIdentity) (GitHubReleaseAccount, error)
	RepositoryID(context.Context, GitHubIssueCLIIdentity, string) (int64, error)
	TagCommit(context.Context, GitHubIssueCLIIdentity, string, string) (string, error)
	Read(context.Context, GitHubIssueCLIIdentity, string, int64, string) (*GitHubReleaseRemote, error)
	CreateDraft(context.Context, GitHubIssueCLIIdentity, GitHubReleaseIntent) (GitHubReleaseRemote, error)
	Publish(context.Context, GitHubIssueCLIIdentity, GitHubReleaseIntent, int64) (GitHubReleaseRemote, error)
}
type GitHubReleaseAccount struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}
type GitHubReleaseRemote struct {
	ID          int64                `json:"id"`
	Tag         string               `json:"tag_name"`
	Commitish   string               `json:"target_commitish"`
	Title       string               `json:"name"`
	Body        string               `json:"body"`
	Draft       bool                 `json:"draft"`
	Prerelease  bool                 `json:"prerelease"`
	URL         string               `json:"html_url"`
	CreatedAt   string               `json:"created_at"`
	PublishedAt *string              `json:"published_at"`
	Author      GitHubReleaseAccount `json:"author"`
}

var githubReleaseTag = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
var githubReleaseSHA = regexp.MustCompile(`^[a-f0-9]{40}$`)
var githubReleaseLogin = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,99}(\[bot\])?$`)
var githubReleaseLinkSlug = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$`)
var errGitHubRelease = errors.New("The original GitHub release, reviewed source or current publishing access could not be confirmed. Read the retained original; no uncertain request is retried.")

type nativeGitHubReleaseRunner struct{ identity nativeGitHubIssuesRunner }
type githubReleaseGuardKey struct{}

func (n nativeGitHubReleaseRunner) Identity(ctx context.Context) (GitHubIssueCLIIdentity, error) {
	return n.identity.Identity(ctx)
}

// Reuses the accepted gh identity/overflow primitives and Pages' explicit HTTP
// status envelope. Neither diagnostic text nor an exit code proves absence.
func (n nativeGitHubReleaseRunner) request(ctx context.Context, id GitHubIssueCLIIdentity, method, endpoint string, input any) (int, []byte, error) {
	guard, ok := ctx.Value(githubReleaseGuardKey{}).(func(context.Context) error)
	if !ok || guard(ctx) != nil {
		return 0, nil, errGitHubRelease
	}
	before, err := n.identity.identity(id.Path)
	if err != nil || before != id {
		return 0, nil, errGitHubRelease
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	args := []string{"api", "--hostname", "github.com", "--method", method, "--include", "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2022-11-28", endpoint}
	var body []byte
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil || len(body) > 48<<10 {
			return 0, nil, errGitHubRelease
		}
		args = append(args, "--input", "-")
	}
	cmd := exec.CommandContext(bounded, id.Path, args...)
	hideProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	if input != nil {
		cmd.Stdin = bytes.NewReader(body)
	}
	env := []string{}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		switch strings.ToUpper(key) {
		case "GH_DEBUG", "GH_HOST", "GH_HTTP_UNIX_SOCKET", "GH_PAGER", "PAGER", "GH_PROMPT_DISABLED", "GIT_TERMINAL_PROMPT":
			continue
		}
		env = append(env, entry)
	}
	cmd.Env = append(env, "GH_PAGER=", "PAGER=", "GH_PROMPT_DISABLED=1", "GIT_TERMINAL_PROMPT=0")
	stdout, stderr := &feedbackOutput{limit: 1 << 20}, &feedbackOutput{limit: 32 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	if guard(bounded) != nil {
		return 0, nil, errGitHubRelease
	}
	runErr := cmd.Run()
	raw, overflow := stdout.snapshot()
	_, errorOverflow := stderr.snapshot()
	if overflow || errorOverflow || bounded.Err() != nil {
		return 0, nil, errGitHubRelease
	}
	after, err := n.identity.identity(id.Path)
	if err != nil || after != id {
		return 0, nil, errGitHubRelease
	}
	head, response, ok := strings.Cut(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n\n")
	first := strings.Fields(strings.SplitN(head, "\n", 2)[0])
	if !ok || len(head) > 32<<10 || len(first) < 2 || !strings.HasPrefix(first[0], "HTTP/") {
		return 0, nil, errGitHubRelease
	}
	status, err := strconv.Atoi(first[1])
	if err != nil || status < 100 || status > 599 || runErr != nil && status >= 200 && status < 300 {
		return 0, nil, errGitHubRelease
	}
	return status, []byte(response), nil
}
func decodeGitHubReleaseJSON(raw []byte, value any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	if d.Decode(value) != nil || d.Decode(new(any)) != io.EOF {
		return errGitHubRelease
	}
	return nil
}
func (n nativeGitHubReleaseRunner) Account(ctx context.Context, id GitHubIssueCLIIdentity) (GitHubReleaseAccount, error) {
	var account GitHubReleaseAccount
	status, raw, err := n.request(ctx, id, "GET", "user", nil)
	if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &account) != nil || account.ID < 1 || account.ID > 9007199254740991 || !githubReleaseLogin.MatchString(account.Login) {
		return account, errGitHubRelease
	}
	return account, nil
}
func (n nativeGitHubReleaseRunner) RepositoryID(ctx context.Context, id GitHubIssueCLIIdentity, repo string) (int64, error) {
	if !validFeedbackRepository(repo) {
		return 0, errGitHubRelease
	}
	status, raw, err := n.request(ctx, id, "GET", "repos/"+repo, nil)
	var value struct {
		ID   int64  `json:"id"`
		Name string `json:"full_name"`
	}
	if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &value) != nil || value.ID < 1 || value.ID > 9007199254740991 || !strings.EqualFold(value.Name, repo) {
		return 0, errGitHubRelease
	}
	return value.ID, nil
}
func (n nativeGitHubReleaseRunner) TagCommit(ctx context.Context, id GitHubIssueCLIIdentity, repo, tag string) (string, error) {
	if !validFeedbackRepository(repo) || !githubReleaseTag.MatchString(tag) {
		return "", errGitHubRelease
	}
	var value struct {
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"object"`
	}
	status, raw, err := n.request(ctx, id, "GET", "repos/"+repo+"/git/ref/tags/"+tag, nil)
	if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &value) != nil {
		return "", errGitHubRelease
	}
	// Annotated tags are followed only through exact SHA objects in THIS repo.
	seen := map[string]bool{}
	for depth := 0; depth < 4; depth++ {
		if !githubReleaseSHA.MatchString(value.Object.SHA) || seen[value.Object.SHA] {
			return "", errGitHubRelease
		}
		seen[value.Object.SHA] = true
		if value.Object.Type == "commit" {
			return value.Object.SHA, nil
		}
		if value.Object.Type != "tag" {
			return "", errGitHubRelease
		}
		status, raw, err = n.request(ctx, id, "GET", "repos/"+repo+"/git/tags/"+value.Object.SHA, nil)
		if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &value) != nil {
			return "", errGitHubRelease
		}
	}
	return "", errGitHubRelease
}
func validGitHubReleaseRemote(row GitHubReleaseRemote, repo, tag string) bool {
	address, err := url.Parse(row.URL)
	parts := strings.Split(strings.TrimPrefix(addressPath(address), "/"), "/")
	// Draft HTML links may use an untagged-* slug. This is a bounded own-repo
	// display link only; exact API ID/tag/author/content carry the correlation.
	routeOK := len(parts) == 5 && (parts[3] == "tag" && githubReleaseLinkSlug.MatchString(parts[4]) || parts[3] == "edit" && parts[4] == strconv.FormatInt(row.ID, 10))
	urlOK := err == nil && address != nil && address.Scheme == "https" && address.Host == "github.com" && address.User == nil && address.RawQuery == "" && !address.ForceQuery && address.Fragment == "" && address.RawPath == "" && routeOK && strings.EqualFold(parts[0]+"/"+parts[1], repo) && parts[2] == "releases"
	return urlOK && row.ID > 0 && row.ID <= 9007199254740991 && row.Author.ID > 0 && githubReleaseLogin.MatchString(row.Author.Login) && row.Tag == tag && githubReleaseTag.MatchString(tag) && validFeedbackRepository(repo) && len(row.Title) > 0 && len(row.Title) <= 200 && len(row.Body) <= 32<<10 && validArtifactStoredText(row.Title) && validArtifactStoredText(row.Body) && feedbackTimestamp(row.CreatedAt) && (row.PublishedAt == nil || feedbackTimestamp(*row.PublishedAt))
}
func addressPath(value *url.URL) string {
	if value == nil {
		return ""
	}
	return value.Path
}
func (n nativeGitHubReleaseRunner) Read(ctx context.Context, id GitHubIssueCLIIdentity, repo string, remoteID int64, tag string) (*GitHubReleaseRemote, error) {
	if !validFeedbackRepository(repo) || !githubReleaseTag.MatchString(tag) || remoteID < 0 || remoteID > 9007199254740991 {
		return nil, errGitHubRelease
	}
	endpoint := "repos/" + repo + "/releases/tags/" + tag
	if remoteID > 0 {
		endpoint = "repos/" + repo + "/releases/" + strconv.FormatInt(remoteID, 10)
	}
	status, raw, err := n.request(ctx, id, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	if status == 404 {
		return nil, nil
	}
	var row GitHubReleaseRemote
	if status != 200 || decodeGitHubReleaseJSON(raw, &row) != nil || !validGitHubReleaseRemote(row, repo, tag) || remoteID > 0 && row.ID != remoteID {
		return nil, errGitHubRelease
	}
	return &row, nil
}
func githubReleasePayload(intent GitHubReleaseIntent, draft bool) map[string]any {
	return map[string]any{"tag_name": intent.Tag, "target_commitish": intent.Commit, "name": intent.Title, "body": intent.Body, "draft": draft, "prerelease": false, "generate_release_notes": false, "make_latest": "false"}
}
func (n nativeGitHubReleaseRunner) CreateDraft(ctx context.Context, id GitHubIssueCLIIdentity, intent GitHubReleaseIntent) (GitHubReleaseRemote, error) {
	if !validFeedbackRepository(intent.Repository) || !githubReleaseTag.MatchString(intent.Tag) || !githubReleaseSHA.MatchString(intent.Commit) {
		return GitHubReleaseRemote{}, errGitHubRelease
	}
	status, raw, err := n.request(ctx, id, "POST", "repos/"+intent.Repository+"/releases", githubReleasePayload(intent, true))
	var row GitHubReleaseRemote
	if err != nil || status != 201 || decodeGitHubReleaseJSON(raw, &row) != nil || !validGitHubReleaseRemote(row, intent.Repository, intent.Tag) || !row.Draft || row.Prerelease || row.Title != intent.Title || row.Body != intent.Body {
		return row, errGitHubRelease
	}
	return row, nil
}
func (n nativeGitHubReleaseRunner) Publish(ctx context.Context, id GitHubIssueCLIIdentity, intent GitHubReleaseIntent, remoteID int64) (GitHubReleaseRemote, error) {
	if !validFeedbackRepository(intent.Repository) || !githubReleaseTag.MatchString(intent.Tag) || remoteID < 1 || remoteID > 9007199254740991 {
		return GitHubReleaseRemote{}, errGitHubRelease
	}
	// Re-send precisely the reviewed copy, not any later remote body. There is
	// no claim that GitHub offers an atomic draft-read/source/grant transaction.
	payload := githubReleasePayload(intent, false)
	delete(payload, "generate_release_notes")
	status, raw, err := n.request(ctx, id, "PATCH", "repos/"+intent.Repository+"/releases/"+strconv.FormatInt(remoteID, 10), payload)
	var row GitHubReleaseRemote
	if err != nil || status != 200 || decodeGitHubReleaseJSON(raw, &row) != nil || !validGitHubReleaseRemote(row, intent.Repository, intent.Tag) || row.ID != remoteID || row.Draft || row.Prerelease || row.PublishedAt == nil || row.Title != intent.Title || row.Body != intent.Body {
		return row, errGitHubRelease
	}
	return row, nil
}
