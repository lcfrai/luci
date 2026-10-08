package registry

import "testing"

func TestSessionStartFailureRejectsUnboundedOrUnrecognizedPublicText(t *testing.T) {
	want := NewSessionStartFailure("admission", "memory_pressure")
	if !want.Valid() {
		t.Fatal("canonical failure invalid")
	}
	for _, change := range []func(*SessionStartFailure){func(f *SessionStartFailure) { f.Message = "secret at /private/path" }, func(f *SessionStartFailure) { f.Stage = "unknown" }, func(f *SessionStartFailure) { f.ReasonCode = "secret-code" }} {
		bad := want
		change(&bad)
		if bad.Valid() {
			t.Fatal("unrecognized diagnostic accepted", bad)
		}
	}
	row := observationSession("failed-start")
	row.StartFailure = want
	clone := row.Clone()
	clone.StartFailure.Message = "caller changed its copy"
	if row.StartFailure != want {
		t.Fatal("caller changed retained diagnostic")
	}
}
