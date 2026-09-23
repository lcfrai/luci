package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// gitGlobalFilesResolver is the production api.Deps.GitGlobalFiles: where the
// host's git looks for its global excludes and attributes files in one
// worktree, asked of that git, in that worktree, so every config scope counts,
// includeIf sections too (gitfix-rereview I-A).
//
// Each is one question to git's own parser:
//
//	git config --get --type=path --default <git's default> core.excludesFile
//	git config --get --type=path --default <git's default> core.attributesFile
//
// --type=path expands "~/" and "%(prefix)/" as the host's git does, with the
// HOME that git sees; Git for Windows sets HOME itself when it is unset. The
// default is git's own for an unset key: $XDG_CONFIG_HOME/git/<name> when that
// variable is set and not empty, else ~/.config/git/<name>. XDG_CONFIG_HOME is
// read from this process's environment, which project.ExecRunner hands to git
// unchanged. A relative answer is joined to the worktree, the directory git
// reads it against, and an empty one, `excludesFile =`, is "" because git then
// reads no file. Whether a file is there is the caller's question.
func gitGlobalFilesResolver(run project.Runner) func(context.Context, string) (api.GitGlobalFiles, error) {
	return gitGlobalFilesResolverWith(run, os.Getenv)
}

// gitGlobalFilesResolverWith is gitGlobalFilesResolver with the environment
// it reads XDG_CONFIG_HOME from as a parameter, so a test can be any host.
func gitGlobalFilesResolverWith(run project.Runner, getenv func(string) string) func(context.Context, string) (api.GitGlobalFiles, error) {
	return func(ctx context.Context, worktreePath string) (api.GitGlobalFiles, error) {
		excludes, err := gitGlobalFile(ctx, run, getenv, worktreePath, "core.excludesFile", "ignore")
		if err != nil {
			return api.GitGlobalFiles{}, err
		}
		attributes, err := gitGlobalFile(ctx, run, getenv, worktreePath, "core.attributesFile", "attributes")
		if err != nil {
			return api.GitGlobalFiles{}, err
		}
		return api.GitGlobalFiles{Excludes: excludes, Attributes: attributes}, nil
	}
}

// gitGlobalFile is the host path git reads for key in dir, or its default
// file <name> in git's per-user config directory.
func gitGlobalFile(ctx context.Context, run project.Runner, getenv func(string) string, dir, key, name string) (string, error) {
	def := "~/.config/git/" + name
	if xdg := getenv("XDG_CONFIG_HOME"); xdg != "" {
		def = xdg + "/git/" + name
	}
	path, err := gitConfigValue(ctx, run, dir, "--type=path", "--default", def, key)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	// An empty value is git reading no file at all (measured with
	// `git -c core.excludesFile= check-ignore`), not a path to join.
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	return path, nil
}
