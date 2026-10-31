package product

import (
	"reflect"
	"strings"
	"testing"
)

func TestFeedbackGitHubOriginAndFixedReadBoundary(t *testing.T) {
	for _, raw := range []string{"https://github.com/Owner/Repo.git", "git@github.com:Owner/Repo.git", "ssh://git@github.com/Owner/Repo"} {
		value, err := canonicalGitHubOrigin(raw)
		if err != nil || value != "owner/repo" {
			t.Fatalf("canonical origin %q: %q %v", raw, value, err)
		}
	}
	for _, raw := range []string{"https://github.com.evil/owner/repo", "https://user@github.com/owner/repo", "https://github.com/owner/repo?token=secret", "ssh://git:secret@github.com/owner/repo", "https://github.com:443/owner/repo", "git@github.com:owner/repo\nother", "https://github.com/owner/%2e%2e", "https://github.com/owner/repo/extra", "https://github.com/owner/.."} {
		if _, err := canonicalGitHubOrigin(raw); err == nil {
			t.Fatalf("unsafe origin accepted: %q", raw)
		}
	}
	args, err := githubIssueArgs("owner/repo")
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"api", "--hostname", "github.com", "--method", "GET", "--header", "Accept: application/vnd.github+json", "--header", "X-GitHub-Api-Version: 2022-11-28", "repos/owner/repo/issues?state=all&per_page=100&sort=updated&direction=desc", "--jq", githubIssueQuery}
	if !reflect.DeepEqual(args, expected) || !strings.Contains(githubIssueQuery, "select(.pull_request==null)") {
		t.Fatal("fixed one-page issue read changed")
	}
	if _, err := githubIssueArgs("owner/repo --method POST"); err == nil {
		t.Fatal("CLI injection accepted")
	}
}

func TestFeedbackOutputDrainsButRefusesTruncatedResult(t *testing.T) {
	output := &feedbackOutput{limit: 4}
	if n, err := output.Write([]byte("123456")); n != 6 || err != nil {
		t.Fatal("overflow stopped draining")
	}
	if n, err := output.Write([]byte("789")); n != 3 || err != nil {
		t.Fatal("subsequent output stopped draining")
	}
	raw, overflow := output.snapshot()
	if string(raw) != "1234" || !overflow {
		t.Fatal("silent truncation became usable output")
	}
	output = &feedbackOutput{limit: 4}
	output.Write([]byte("1234"))
	_, overflow = output.snapshot()
	if overflow {
		t.Fatal("exact limit falsely overflowed")
	}
}
