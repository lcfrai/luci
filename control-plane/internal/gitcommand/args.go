// Package gitcommand supplies platform settings shared by Git process boundaries.
package gitcommand

import (
	"path/filepath"
	"runtime"
	"strings"
)

// PlatformArgs enables Git for Windows' long-path support for this invocation.
// Managed user roots and linked worktrees can exceed MAX_PATH even when each
// project name is short. Do not depend on or modify the person's global config.
// Existing repository confinement and command restrictions remain the caller's.
func PlatformArgs(dir string, args []string) []string {
	if runtime.GOOS != "windows" {
		return args
	}
	args = append([]string{}, args...)
	for i := 0; i < len(args); i++ {
		if args[i] == "--" || !strings.HasPrefix(args[i], "-") {
			break
		}
		if args[i] == "-C" || strings.HasPrefix(args[i], "-C") {
			// A caller-selected chdir changes relative option semantics. Our
			// managed callers use cmd.Dir, but leave other commands unchanged.
			dir = ""
			break
		}
		if args[i] == "-c" || args[i] == "--config-env" {
			i++
		}
	}
	// Git for Windows separately rejects an explicit absolute --git-dir
	// longer than PATH_MAX-40 before consulting longpaths. Preserve the exact
	// validated location using its equivalent path relative to cmd.Dir.
	// Only inspect Git's global options, never subcommand arguments or paths.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || !strings.HasPrefix(arg, "-") {
			break
		}
		if arg == "-c" || arg == "-C" || arg == "--config-env" {
			i++
			continue
		}
		if gitDir, ok := strings.CutPrefix(arg, "--git-dir="); ok && filepath.IsAbs(dir) && filepath.IsAbs(gitDir) {
			if relative, err := filepath.Rel(dir, gitDir); err == nil {
				args[i] = "--git-dir=" + filepath.ToSlash(relative)
			}
		}
	}
	// Git copies init/clone templates before its repository configuration is
	// ready. Even with longpaths requested, a sample hook can exceed MAX_PATH
	// during that early phase. Managed repositories need no host templates.
	return append([]string{"-c", "core.longpaths=true", "-c", "init.templateDir="}, args...)
}
