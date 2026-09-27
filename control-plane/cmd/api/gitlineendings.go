package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/api"
)

// gitLineEndingsResolver is the production api.Deps.GitLineEndings: how the
// host's git converts line endings, from the host's own config and never a
// worktree's (hostGit; X-1). That is the system scope a container never sees,
// where Git for Windows' installer writes core.autocrlf=true, and the global
// scope over it. A worktree's .git/config is the agent's to write, so its
// core.autocrlf and core.eol are not asked: the sandbox gets the host's own
// value, as command-scope config that outranks the repository's (phase 3
// ruling R4 rule 6). A repository's .gitattributes still applies in the
// sandbox as it does on the host, because attributes outrank both keys.
//
// Two questions, each answered by git's own parser rather than a copy of it,
// in the scope hostGit.scope finds:
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
func gitLineEndingsResolver(host hostGit) func(context.Context) (api.GitLineEndings, error) {
	return gitLineEndingsResolverWith(host, hostNativeEOL())
}

// gitLineEndingsResolverWith is gitLineEndingsResolver on a host whose git
// reads "native" as native.
func gitLineEndingsResolverWith(host hostGit, native string) func(context.Context) (api.GitLineEndings, error) {
	return func(ctx context.Context) (api.GitLineEndings, error) {
		var out api.GitLineEndings
		// One budget for the whole read, sized for the read; see
		// hostConfigReadBudget.
		err := host.read(ctx, hostConfigReadBudget, func(ctx context.Context) error {
			scope, err := host.scope(ctx, "core.autocrlf")
			if err != nil {
				return fmt.Errorf("read core.autocrlf: %w", err)
			}
			autocrlf, err := host.config(ctx, scope, "--type=bool", "--default", "false", "core.autocrlf")
			if err != nil {
				raw, rawErr := host.config(ctx, scope, "core.autocrlf")
				if rawErr != nil || !strings.EqualFold(raw, "input") {
					return fmt.Errorf("read core.autocrlf: %w", err)
				}
				autocrlf = "input"
			}
			eol, err := host.value(ctx, "core.eol", "--default", "native")
			if err != nil {
				return fmt.Errorf("read core.eol: %w", err)
			}
			switch strings.ToLower(eol) {
			case "lf", "crlf":
				eol = strings.ToLower(eol)
			default:
				eol = native
			}
			out = api.GitLineEndings{AutoCRLF: autocrlf, EOL: eol}
			return nil
		})
		if err != nil {
			return api.GitLineEndings{}, err
		}
		return out, nil
	}
}

// hostNativeEOL is the line ending "native" means to this host's git: Git for
// Windows is built with NATIVE_CRLF, and git everywhere else is not.
func hostNativeEOL() string {
	if runtime.GOOS == "windows" {
		return "crlf"
	}
	return "lf"
}
