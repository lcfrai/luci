package proxy

import (
	"strings"
	"testing"
)

func TestProcessExitDecoderRetainsOnlyClosedCodeAndSignal(t *testing.T) {
	for _, test := range []struct{ payload, reason string }{
		{"exit status 0", "process exited with code 0"},
		{"exit status 1", "process exited with code 1"},
		{"exit status 137", "process exited with code 137"},
		{"exit status 255", "process exited with code 255"},
		{"signal: killed", "process ended after SIGKILL; cause unconfirmed"},
		{"signal: terminated", "process ended after SIGTERM; cause unconfirmed"},
		{"signal: segmentation fault (core dumped)", "process ended after SIGSEGV; cause unconfirmed"},
		{"signal: aborted", "process ended after SIGABRT; cause unconfirmed"},
	} {
		t.Run(test.payload, func(t *testing.T) {
			exit := DecodeProcessExit([]byte(test.payload))
			if !exit.Known() || exit.Reason() != test.reason || strings.Contains(exit.Reason(), "out of memory") {
				t.Fatal("process result lost its closed diagnostic or invented a cause", exit.Reason())
			}
		})
	}
}

func TestProcessExitDecoderRejectsUntrustedOrAmbiguousPayload(t *testing.T) {
	for _, payload := range []string{"", "exit status -1", "exit status 256", "exit status 01", "exit status +1", "exit status 1\n", " exit status 1", "exit status 1 private-key", "exit status ١", "exit status 1\x00", "signal: private-provider-error", "signal: killed\nprivate", "signal: killed (core dumped) trailing", "provider failed: /private/auth.json", strings.Repeat("x", 65)} {
		t.Run(payload, func(t *testing.T) {
			exit := DecodeProcessExit([]byte(payload))
			if exit.Known() || exit.Reason() != "process exited" {
				t.Fatal("untrusted payload became public lifecycle metadata", exit.Reason())
			}
		})
	}
}
