package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// gitLineEndingsResolver is the production api.Deps.GitLineEndings: how the
// host's git converts line endings in one worktree, asked of that git, in that
// worktree, so every scope it reads counts. That includes the system scope a
// container never sees, where Git for Windows' installer writes
// core.autocrlf=true, and a repository's own .git/config and includeIf
// sections.
//
// Two questions, each answered by git's own parser rather than a copy of it:
//
//  1. core.autocrlf, as git reads a boolean: `git config --type=bool`, with
//     --default false for unset, which is git's own default. That turns
//     "yes", "on", "1" and a bare key into "true", the way the host's git
//     reads them. It refuses "input", the one word that is not a boolean, so
//     only then is the raw value read; anything else it refuses is a value
//     the host's git dies on too, and the error carries git's sentence.
//  2. core.eol, with --default native. "lf" and "crlf" are passed on; git
//     reads any other value, "native" included, as native, which is the
//     host's ending: native.
//
// native is hostNativeEOL(), chosen here rather than by the caller so that
// serve(), which no unit test runs, has nothing to get wrong.
// gitLineEndingsResolverWith takes it as a parameter so a test can be both
// hosts.
func gitLineEndingsResolver(run project.Runner) func(context.Context, string) (api.GitLineEndings, error) {
	return gitLineEndingsResolverWith(run, hostNativeEOL())
}

// gitLineEndingsResolverWith is gitLineEndingsResolver on a host whose git
// reads "native" as native.
func gitLineEndingsResolverWith(run project.Runner, native string) func(context.Context, string) (api.GitLineEndings, error) {
	return func(ctx context.Context, worktreePath string) (api.GitLineEndings, error) {
		autocrlf, err := gitConfigValue(ctx, run, worktreePath, "--type=bool", "--default", "false", "core.autocrlf")
		if err != nil {
			raw, rawErr := gitConfigValue(ctx, run, worktreePath, "core.autocrlf")
			if rawErr != nil || !strings.EqualFold(raw, "input") {
				return api.GitLineEndings{}, fmt.Errorf("read core.autocrlf: %w", err)
			}
			autocrlf = "input"
		}
		eol, err := gitConfigValue(ctx, run, worktreePath, "--default", "native", "core.eol")
		if err != nil {
			return api.GitLineEndings{}, fmt.Errorf("read core.eol: %w", err)
		}
		switch strings.ToLower(eol) {
		case "lf", "crlf":
			eol = strings.ToLower(eol)
		default:
			eol = native
		}
		return api.GitLineEndings{AutoCRLF: autocrlf, EOL: eol}, nil
	}
}

// gitConfigValue runs `git config --get <args...>` in dir and returns its one
// line of output.
func gitConfigValue(ctx context.Context, run project.Runner, dir string, args ...string) (string, error) {
	out, err := run.Run(ctx, dir, append([]string{"config", "--get"}, args...)...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// hostNativeEOL is the line ending "native" means to this host's git: Git for
// Windows is built with NATIVE_CRLF, and git everywhere else is not.
func hostNativeEOL() string {
	if runtime.GOOS == "windows" {
		return "crlf"
	}
	return "lf"
}
