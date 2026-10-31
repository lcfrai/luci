package proxy

import "testing"

func TestProcessExitSuccessfulRequiresKnownTypedZero(t *testing.T) {
	for _, payload := range []string{"exit status 0", "exit status 1", "signal: killed", "unknown", "stdout exit status 0", "exit status 00", "exit status 0\n"} {
		if got := DecodeProcessExit([]byte(payload)).Successful(); got != (payload == "exit status 0") {
			t.Fatalf("typed success mismatch %q: %v", payload, got)
		}
	}
	if (ProcessExit{}).Successful() {
		t.Fatal("zero value invented success")
	}
}
