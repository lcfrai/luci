package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

var ErrProjectFeedback = errors.New("Project feedback could not be confirmed. Refresh the saved context and review the selected issues again; no publication or outreach was performed.")
var ErrGitHubIssueOverflow = errors.New("The GitHub response exceeded this preview's bounded output. No partial preview was retained.")

// The injectable boundary accepts only a canonical server-derived repository,
// never a URL, command, token, arbitrary arguments or an execution capability.
type GitHubIssuesRunner interface {
	Identity(context.Context) (GitHubIssueCLIIdentity, error)
	ReadIssues(context.Context, GitHubIssueCLIIdentity, string) (GitHubIssuePage, error)
}
type GitHubIssueCLIIdentity struct {
	Path   string `json:"path"`
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type GitHubIssuePage struct {
	Fetched int           `json:"fetched"`
	Issues  []GitHubIssue `json:"issues"`
}
type GitHubIssue struct {
	Number        int    `json:"number"`
	Title         string `json:"title"`
	Body          string `json:"body"`
	BodyTruncated bool   `json:"bodyTruncated"`
	State         string `json:"state"`
	UpdatedAt     string `json:"updatedAt"`
}

func canonicalGitHubOrigin(raw string) (string, error) {
	if raw == "" || len(raw) > 2048 || strings.ContainsAny(raw, "\r\n\x00\\ \t") {
		return "", ErrProjectFeedback
	}
	path := ""
	if strings.HasPrefix(raw, "git@github.com:") {
		path = strings.TrimPrefix(raw, "git@github.com:")
	} else {
		u, err := url.Parse(raw)
		if err != nil || !strings.EqualFold(u.Host, "github.com") || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
			return "", ErrProjectFeedback
		}
		if u.Scheme == "https" {
			if u.User != nil {
				return "", ErrProjectFeedback
			}
		} else if u.Scheme == "ssh" {
			if u.User == nil || u.User.Username() != "git" {
				return "", ErrProjectFeedback
			}
			if _, present := u.User.Password(); present {
				return "", ErrProjectFeedback
			}
		} else {
			return "", ErrProjectFeedback
		}
		path = strings.TrimPrefix(u.Path, "/")
	}
	pieces := strings.Split(path, "/")
	if len(pieces) != 2 {
		return "", ErrProjectFeedback
	}
	pieces[1] = strings.TrimSuffix(pieces[1], ".git")
	if !githubOwner.MatchString(pieces[0]) || !githubRepo.MatchString(pieces[1]) || pieces[1] == "." || pieces[1] == ".." {
		return "", ErrProjectFeedback
	}
	return strings.ToLower(pieces[0] + "/" + pieces[1]), nil
}

func validFeedbackRepository(repo string) bool {
	canonical, err := canonicalGitHubOrigin("https://github.com/" + repo)
	return err == nil && canonical == repo
}

const githubIssueQuery = `{fetched:length,issues:[.[]|select(.pull_request==null)|{number,title,body:((.body//"")[:2048]),bodyTruncated:(((.body//"")|length)>2048),state,updatedAt:.updated_at}]}`

func githubIssueArgs(repository string) ([]string, error) {
	if !validFeedbackRepository(repository) {
		return nil, ErrProjectFeedback
	}
	return []string{"api", "--hostname", "github.com", "--method", "GET", "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2022-11-28", "repos/" + repository + "/issues?state=all&per_page=100&sort=updated&direction=desc", "--jq", githubIssueQuery}, nil
}

type feedbackOutput struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	limit    int
	overflow bool
}

func (b *feedbackOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(p)
	remaining := b.limit - b.buffer.Len()
	if len(p) > remaining {
		b.overflow = true
		p = p[:max(remaining, 0)]
	}
	_, _ = b.buffer.Write(p)
	return n, nil
}
func (b *feedbackOutput) snapshot() ([]byte, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]byte(nil), b.buffer.Bytes()...), b.overflow
}

type nativeGitHubIssuesRunner struct{ projectsRoot func() string }

// gh retains its existing workspace authentication. This is not an assertion
// that the connected GitHub account belongs to the current Luci person.
func (n nativeGitHubIssuesRunner) identity(path string) (GitHubIssueCLIIdentity, error) {
	if !filepath.IsAbs(path) || runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(path), ".exe") {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	if n.projectsRoot == nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	if _, err := hostpath.ResolveOutside(n.projectsRoot(), path); err != nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	if err := hostpath.OneName(path); err != nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Size() <= 0 || before.Size() > 128<<20 {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	file, err := os.Open(path)
	if err != nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	first, firstErr := file.Stat()
	hash := sha256.New()
	read, readErr := io.Copy(hash, io.LimitReader(file, before.Size()+1))
	last, lastErr := file.Stat()
	closeErr := file.Close()
	after, afterErr := os.Lstat(path)
	if firstErr != nil || readErr != nil || lastErr != nil || closeErr != nil || afterErr != nil || read != before.Size() || !os.SameFile(before, first) || !os.SameFile(first, last) || !os.SameFile(last, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	return GitHubIssueCLIIdentity{Path: path, Bytes: read, SHA256: hex.EncodeToString(hash.Sum(nil))}, nil
}
func (n nativeGitHubIssuesRunner) Identity(ctx context.Context) (GitHubIssueCLIIdentity, error) {
	if ctx.Err() != nil {
		return GitHubIssueCLIIdentity{}, ctx.Err()
	}
	path, err := exec.LookPath("gh")
	if err != nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return GitHubIssueCLIIdentity{}, ErrProjectFeedback
	}
	return n.identity(absolute)
}

func (n nativeGitHubIssuesRunner) ReadIssues(ctx context.Context, identity GitHubIssueCLIIdentity, repository string) (GitHubIssuePage, error) {
	args, err := githubIssueArgs(repository)
	if err != nil {
		return GitHubIssuePage{}, err
	}
	before, err := n.identity(identity.Path)
	if err != nil || before != identity {
		return GitHubIssuePage{}, ErrProjectFeedback
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(bounded, identity.Path, args...)
	hideProcess(cmd)
	cmd.WaitDelay = 2 * time.Second
	// Preserve existing CLI auth without inspecting or returning its secrets.
	// Suppress diagnostics/pagers and alternate-host/socket overrides only.
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
	stdout, stderr := &feedbackOutput{limit: 2 << 20}, &feedbackOutput{limit: 32 << 10}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	raw, overflow := stdout.snapshot()
	_, errorOverflow := stderr.snapshot()
	if overflow || errorOverflow {
		return GitHubIssuePage{}, ErrGitHubIssueOverflow
	}
	if runErr != nil || bounded.Err() != nil {
		return GitHubIssuePage{}, ErrProjectFeedback
	}
	after, err := n.identity(identity.Path)
	if err != nil || after != identity {
		return GitHubIssuePage{}, ErrProjectFeedback
	}
	var page GitHubIssuePage
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&page) != nil || decoder.Decode(new(any)) != io.EOF || !validGitHubIssuePage(page) {
		return GitHubIssuePage{}, ErrProjectFeedback
	}
	return page, nil
}

func validGitHubIssuePage(page GitHubIssuePage) bool {
	if page.Fetched < 0 || page.Fetched > 100 || page.Issues == nil || len(page.Issues) > page.Fetched {
		return false
	}
	seen := map[int]bool{}
	for _, issue := range page.Issues {
		if issue.Number <= 0 || issue.Number > 1_000_000_000 || seen[issue.Number] || strings.TrimSpace(issue.Title) == "" || len(issue.Title) > 2048 || len(issue.Body) > 8192 || !validArtifactStoredText(issue.Title) || !validArtifactStoredText(issue.Body) || issue.State != "open" && issue.State != "closed" || !feedbackTimestamp(issue.UpdatedAt) {
			return false
		}
		seen[issue.Number] = true
	}
	return true
}
