package main

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

type configReadFunc func(context.Context, string, ...string) ([]byte, error)

func (read configReadFunc) Run(ctx context.Context, dir string, args ...string) ([]byte, error) {
	return read(ctx, dir, args...)
}

func TestHostConfigDefaultCannotReadASettingAddedAfterTheMissingAnswer(t *testing.T) {
	calls := 0
	host := hostGit{root: t.TempDir(), run: configReadFunc(func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			want := []string{"config", "-z", "--show-origin", "--get", "--global", "--includes", "--type=path", "core.excludesFile"}
			if !reflect.DeepEqual(args, want) {
				t.Fatalf("configured query = %q; must never combine origin and default", args)
			}
			return nil, configUnsetAnswer()
		}
		// A project-controlled include can acquire a setting between reads.
		// The fallback must not consult global/system config at all: no origin
		// was checked for that new setting. Read Git's parser on an empty file.
		want := []string{"config", "-z", "--get", "--file", os.DevNull, "--type=path", "--default", "~/.config/git/ignore", "core.excludesFile"}
		if calls != 2 || !reflect.DeepEqual(args, want) {
			t.Fatalf("default query could adopt an unchecked setting: %q", args)
		}
		return []byte("/owned/home/.config/git/ignore\x00"), nil
	})}
	value, origin, err := host.valueWithOrigin(context.Background(), "--global", "core.excludesFile", "--type=path", "--default", "~/.config/git/ignore")
	if err != nil || value != "/owned/home/.config/git/ignore" || origin != "" || calls != 2 {
		t.Fatalf("default = %q, origin %q, calls %d, err %v", value, origin, calls, err)
	}
}

func TestHostConfigDefaultDoesNotAdoptProcessFailureAsAnUnsetSetting(t *testing.T) {
	for name, failure := range map[string]error{
		"never started":                  &project.GitError{Err: configExitCode(1)},
		"not a Git-owned nonzero answer": &project.GitError{Err: configExitCode(1), Exited: true},
		"timeout":                        &project.GitError{Err: configExitCode(1), Exited: true, ExitedNonZero: true, TimedOut: true},
		"cancellation":                   &project.GitError{Err: configExitCode(1), Exited: true, ExitedNonZero: true, Canceled: true},
		"another quiet exit code":        &project.GitError{Err: configExitCode(2), Exited: true, ExitedNonZero: true},
		"Git abort":                      &project.GitError{Err: configExitCode(134), Exited: true, ExitedNonZero: true, Stderr: "BUG: current_config_origin_type called outside config callback"},
		"error with stderr":              &project.GitError{Err: configExitCode(1), Exited: true, ExitedNonZero: true, Stderr: "invalid key"},
		"error with partial stdout":      &project.GitError{Err: configExitCode(1), Exited: true, ExitedNonZero: true, Stdout: "partial"},
		"no actual exit code":            &project.GitError{Err: errors.New("exit status 1"), Exited: true, ExitedNonZero: true},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			host := hostGit{root: t.TempDir(), run: configReadFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) { calls++; return nil, failure })}
			value, origin, err := host.valueWithOrigin(context.Background(), "--global", "core.excludesFile", "--type=path", "--default", "~/.config/git/ignore")
			if err != failure || value != "" || origin != "" || calls != 1 {
				t.Fatalf("failure was adopted as a default: %q %q %v, %d calls", value, origin, err, calls)
			}
		})
	}
}

func TestHostConfigConfiguredValueKeepsItsIncludedFileOrigin(t *testing.T) {
	calls := 0
	host := hostGit{root: t.TempDir(), run: configReadFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		calls++
		return []byte(fileOrigin("/projects/dotfiles/include.inc", "/outside/ignored")), nil
	})}
	value, origin, err := host.valueWithOrigin(context.Background(), "--global", "core.excludesFile", "--type=path", "--default", "~/.config/git/ignore")
	if err != nil || value != "/outside/ignored" || origin != "/projects/dotfiles/include.inc" || calls != 1 {
		t.Fatalf("included provenance lost: %q %q %v, %d calls", value, origin, err, calls)
	}
}

func TestHostConfigDefaultRequiresOneCompleteValueAndNoConfiguredOriginGuess(t *testing.T) {
	for name, answer := range map[string]string{"no terminator": "/home/ignore", "multiple values": "/one\x00/two\x00", "origin record": "command line:\x00/home/ignore\x00"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			host := hostGit{root: t.TempDir(), run: configReadFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
				calls++
				if calls == 1 {
					return nil, configUnsetAnswer()
				}
				return []byte(answer), nil
			})}
			if value, origin, err := host.valueWithOrigin(context.Background(), "--global", "core.excludesFile", "--type=path", "--default", "~/.config/git/ignore"); err == nil || value != "" || origin != "" {
				t.Fatalf("malformed default accepted: %q %q %v", value, origin, err)
			}
		})
	}
	host := hostGit{root: t.TempDir(), run: configReadFunc(func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		return []byte(cmdOrigin("/unchecked/value")), nil
	})}
	if _, _, err := host.valueWithOrigin(context.Background(), "--global", "core.excludesFile", "--type=path", "--default", "~/.config/git/ignore"); err == nil {
		t.Fatal("configured command-line origin was adopted without a file provenance")
	}
}
