package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/api"
)

// gitGlobalFilesResolver is the production api.Deps.GitGlobalFiles: where the
// host's git looks for its global excludes and attributes files, from the
// host's own config and never a worktree's (hostGit; X-1). What it names is a
// CANDIDATE and not yet a mount, in two ways that have each been a defect. A
// worktree's .git/config, which the agent writes, must not be able to name it
// at all (X-1). And the session refuses the candidate outright when REACHING
// it passes through the projects directory, because a link or junction hop
// there is the agent's to swap (X-3, X-1 review m-5; the X-3 fix round added the
// junction, a reparse point EvalSymlinks does not resolve) — it fails the create rather than
// falling back, which is why this resolver still answers what the host's git
// reads and leaves the ruling on that answer to the session.
//
// Each is one question to git's own parser, in the scope hostGit.scope finds:
//
//	git config -z --show-origin --get --global|--system --includes --type=path core.excludesFile
//	git config -z --show-origin --get --global|--system --includes --type=path core.attributesFile
//
// -z --show-origin pairs each configured value with the file that set it, its name
// raw (valueWithOrigin; X-3 review 3 and 4 I-C); an answer that is not one such
// record is an error, which fails the create. A quiet Git exit 1 proves an
// unset key; Git expands its default in a separate read of the empty OS null
// file, avoiding --show-origin/--default crashes and unchecked config races.
//
// --type=path expands "~/" and "%(prefix)/" as the host's git does, with the
// HOME that git sees; Git for Windows sets HOME itself when it is unset. The
// default is git's own for an unset key: $XDG_CONFIG_HOME/git/<name> when that
// variable is set and not empty, else ~/.config/git/<name>. XDG_CONFIG_HOME is
// read from this process's environment, which project.ExecRunner hands to git
// unchanged. An empty answer, `excludesFile =`, is "" because git then reads
// no file. A relative answer is passed on as git gave it: git reads it against
// each repository, a directory the agent writes, so it names no host file, and
// the session never mounts it: inside the repository the sandbox gets it as it
// is, and outside it fails the create (X-1 review m-4). Whether a file is
// there is the caller's question.
func gitGlobalFilesResolver(host hostGit) func(context.Context) (api.GitGlobalFiles, error) {
	return func(ctx context.Context) (api.GitGlobalFiles, error) {
		var out api.GitGlobalFiles
		// One budget for the whole read — this is the read with the most gits,
		// up to six including absent-key defaults, and the one whose failure
		// is on record (progress.md:6602).
		err := host.read(ctx, hostConfigReadBudget, func(ctx context.Context) error {
			excludes, exclOrigin, err := gitGlobalFile(ctx, host, "core.excludesFile", "ignore")
			if err != nil {
				return err
			}
			attributes, attrOrigin, err := gitGlobalFile(ctx, host, "core.attributesFile", "attributes")
			if err != nil {
				return err
			}
			var origins []string
			for _, o := range []string{exclOrigin, attrOrigin} {
				if o != "" {
					origins = append(origins, o)
				}
			}
			out = api.GitGlobalFiles{Excludes: excludes, Attributes: attributes, Origins: origins}
			return nil
		})
		if err != nil {
			return api.GitGlobalFiles{}, err
		}
		return out, nil
	}
}

// gitGlobalFile is the host path the host's own config sets for key, or its
// default file <name> in git's per-user config directory, AND — when that path
// is absolute, so a host file would be mounted — the config file that set it, so
// the session can check that file's provenance (X-3 review 3 I-C). The origin is
// "" for a relative value (it names a file in each repository, not a host file)
// and for an unset value (the separate empty-null read expands only the
// default; no config file set it).
func gitGlobalFile(ctx context.Context, host hostGit, key, name string) (value, origin string, err error) {
	def := "~/.config/git/" + name
	if xdg := host.getenv("XDG_CONFIG_HOME"); xdg != "" {
		def = xdg + "/git/" + name
	}
	scope, err := host.scope(ctx, key)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", key, err)
	}
	value, origin, err = host.valueWithOrigin(ctx, scope, key, "--type=path", "--default", def)
	if err != nil {
		return "", "", fmt.Errorf("read %s: %w", key, err)
	}
	// An empty value is git reading no file at all (measured with
	// `git -c core.excludesFile= check-ignore`), not a path; a relative one
	// names a file in each repository, not a host file. Neither mounts a host
	// file, so neither carries a provenance the session must check.
	if !filepath.IsAbs(value) {
		return value, "", nil
	}
	return value, origin, nil
}
