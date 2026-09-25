package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

// gitIdentityResolver is the production api.Deps.GitIdentity: who an agent's
// commits in a worktree are made as. Users authenticate as themselves
// (CLAUDE.md), so the answer is always the person running this control plane,
// and there are exactly two ways to say who that is:
//
//  1. PANACEA_APP_GIT_NAME and PANACEA_APP_GIT_EMAIL, which loadConfig has
//     already checked are a pair. They win outright; the host is never asked.
//  2. Otherwise, the identity the host's own git would stamp on a commit
//     outside any repository, read with `git var GIT_AUTHOR_IDENT` in
//     hostGit's directory, never a worktree (X-1, phase 3 ruling R4 rule 6).
//
// Why `git var` and not `git config user.name`: it is git's own answer, in
// git's own order — environment, global and system config, then the name and
// address git derives from the account. The last step is not hypothetical. On
// the machine this was written on no scope sets user.name at all, and every
// commit its owner makes is by the identity git derives, so reading config
// alone would find nobody. A worktree's own user.name is not asked: its
// .git/config is the agent's to write, and the commits are the user's.
//
// When neither way yields an identity the error says so in a sentence naming
// both fixes, and git's own words ride along. The session still starts.
func gitIdentityResolver(name, email string, host hostGit) func(context.Context) (api.GitIdentity, error) {
	if name != "" && email != "" {
		fixed := api.GitIdentity{Name: name, Email: email}
		return func(context.Context) (api.GitIdentity, error) { return fixed, nil }
	}
	return func(ctx context.Context) (api.GitIdentity, error) {
		out, err := host.git(ctx, "var", "GIT_AUTHOR_IDENT")
		if err != nil {
			return api.GitIdentity{}, fmt.Errorf("%s: %w", gitIdentityMissing, err)
		}
		id, ok := parseGitIdent(string(out))
		if !ok {
			return api.GitIdentity{}, fmt.Errorf("%s: git var GIT_AUTHOR_IDENT printed %q, which is not \"Name <email> time zone\"", gitIdentityMissing, strings.TrimSpace(string(out)))
		}
		return id, nil
	}
}

// gitIdentityMissing is the sentence a user acts on. It names both fixes
// because either works, and the second is the one git itself suggests.
var gitIdentityMissing = "no git identity, so a commit in a sandbox will fail: set " +
	envGitName + " and " + envGitEmail + ", or give this machine's git an identity with " +
	"git config --global user.name and user.email, then start the session again"

// parseGitIdent reads git's ident line, "Name <email> 1790156728 +1000". Git
// strips angle brackets from both parts, so the first '<' and the first '>'
// after it are the email's delimiters. An empty part is not an identity.
func parseGitIdent(line string) (api.GitIdentity, bool) {
	line = strings.TrimSpace(line)
	lt := strings.IndexByte(line, '<')
	if lt < 0 {
		return api.GitIdentity{}, false
	}
	gt := strings.IndexByte(line[lt:], '>')
	if gt < 0 {
		return api.GitIdentity{}, false
	}
	id := api.GitIdentity{
		Name:  strings.TrimSpace(line[:lt]),
		Email: strings.TrimSpace(line[lt+1 : lt+gt]),
	}
	if id.Name == "" || id.Email == "" {
		return api.GitIdentity{}, false
	}
	return id, true
}
