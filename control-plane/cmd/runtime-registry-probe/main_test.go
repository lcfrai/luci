package main

import (
	"strings"
	"testing"
)

func TestProbeRefusesRemoteAndForeignRoutesBeforeEffects(t *testing.T) {
	for _, endpoint := range []string{"tcp://127.0.0.1:2375", "https://remote:2376", "unix:///tmp/foreign.sock", "npipe:////./pipe/foreign"} {
		if validate(endpoint, "engine", strings.Repeat("a", 32), 19379) == nil {
			t.Fatalf("accepted foreign transport %q", endpoint)
		}
	}
	for _, session := range []string{"", strings.Repeat("A", 32), strings.Repeat("a", 31), "../../other"} {
		if validate("npipe:////./pipe/dockerDesktopLinuxEngine", "engine", session, 19379) == nil {
			t.Fatalf("accepted foreign session %q", session)
		}
	}
	for _, port := range []int{0, 80, 65536} {
		if validate("unix:///var/run/docker.sock", "engine", strings.Repeat("a", 32), port) == nil {
			t.Fatalf("accepted port %d", port)
		}
	}
	if err := validate("npipe:////./pipe/dockerDesktopLinuxEngine", "engine", strings.Repeat("a", 32), 19379); err != nil {
		t.Fatal(err)
	}
}
