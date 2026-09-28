package binding_test

// The stand-ins this package's tests hold phase 7 against. Every one of them
// exists because the real thing has not landed in this tree: phase 3 task 13's
// githost, and Hub H0's hubclient and hubapi. They live in _test.go files on
// purpose - binding must never own URL canonicalisation, nor a copy of the
// Hub's key rule, and a stand-in in production code is exactly how a fourth
// canonicaliser gets written.

import (
	"context"
	"net/url"
	"strings"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// testParse stands in for githost.ParseRemote. It is only good enough to prove
// that binding matches on the KEY and never on the URL: the SSH and HTTPS forms
// of one GitHub repository give one key, and dev.azure.com and the old
// .visualstudio.com form of one Azure DevOps repository give one key.
func testParse(raw string) (binding.Remote, error) {
	s := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), ".git"))
	for _, p := range []string{"https://", "http://", "ssh://", "git@"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.TrimPrefix(s, "git@")
	s = strings.ReplaceAll(s, "github.com:", "github.com/")
	switch {
	case strings.HasPrefix(s, "github.com/"):
		if len(strings.Split(s, "/")) == 3 {
			return binding.Remote{RepoKey: s}, nil
		}
	case strings.HasPrefix(s, "dev.azure.com/"):
		p := strings.Split(strings.TrimPrefix(s, "dev.azure.com/"), "/")
		if len(p) == 4 && p[2] == "_git" {
			return binding.Remote{RepoKey: adoKey(p[0], p[1], p[3])}, nil
		}
	case strings.Contains(s, ".visualstudio.com/"):
		org, rest, _ := strings.Cut(s, ".visualstudio.com/")
		p := strings.Split(rest, "/")
		if len(p) == 3 && p[1] == "_git" {
			return binding.Remote{RepoKey: adoKey(org, p[0], p[2])}, nil
		}
	}
	return binding.Remote{}, binding.ErrUnknownHost
}

func adoKey(org, proj, repo string) string {
	if un, err := url.PathUnescape(proj); err == nil {
		proj = un
	}
	return "azure-devops/" + org + "/" + proj + "/" + repo
}

// fakeRemotes answers OriginURL from a path -> url map. A path it does not hold
// has no origin, which is the NoOrigin case, not an error.
type fakeRemotes struct {
	mu      sync.Mutex
	origins map[string]string
	err     error
	calls   int
}

func (f *fakeRemotes) OriginURL(_ context.Context, path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	return f.origins[path], nil
}

// fakeLocals is the phase 2 side: the local projects and their paths.
type fakeLocals struct {
	mu    sync.Mutex
	list  []project.Project
	err   error
	calls int
}

func (f *fakeLocals) Projects(context.Context) ([]project.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return append([]project.Project(nil), f.list...), nil
}
