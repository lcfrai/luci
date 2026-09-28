package main

import (
	"context"
	"fmt"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
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
//	git config --get --global|--system --includes --type=path --default <git's default> core.excludesFile
//	git config --get --global|--system --includes --type=path --default <git's default> core.attributesFile
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
		// four, and the one whose failure is on record (progress.md:6602).
		err := host.read(ctx, hostConfigReadBudget, func(ctx context.Context) error {
			excludes, err := gitGlobalFile(ctx, host, "core.excludesFile", "ignore")
			if err != nil {
				return err
			}
			attributes, err := gitGlobalFile(ctx, host, "core.attributesFile", "attributes")
			if err != nil {
				return err
			}
			out = api.GitGlobalFiles{Excludes: excludes, Attributes: attributes}
			return nil
		})
		if err != nil {
			return api.GitGlobalFiles{}, err
		}
		return out, nil
	}
}

// gitGlobalFile is the host path the host's own config sets for key, or its
// default file <name> in git's per-user config directory.
func gitGlobalFile(ctx context.Context, host hostGit, key, name string) (string, error) {
	def := "~/.config/git/" + name
	if xdg := host.getenv("XDG_CONFIG_HOME"); xdg != "" {
		def = xdg + "/git/" + name
	}
	path, err := host.value(ctx, key, "--type=path", "--default", def)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	// An empty value is git reading no file at all (measured with
	// `git -c core.excludesFile= check-ignore`), not a path.
	return path, nil
}
