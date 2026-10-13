package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCloudOperatorConfigRejectsPublicLinkedAndUntrustedOrigins(t *testing.T) {
	root := t.TempDir()
	private := filepath.Join(root, "operator.json")
	if err := os.WriteFile(private, []byte(`{"scope":"owned"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if data, err := readPrivateE2BFile(private, 1024); err != nil || string(data) != `{"scope":"owned"}` {
		t.Fatal("private operator config was unreadable", err)
	}
	if _, err := readPrivateE2BFile(private, 4); err == nil {
		t.Fatal("oversized operator config was accepted")
	}
	linked := filepath.Join(root, "linked.json")
	if err := os.Symlink(private, linked); err == nil {
		if _, err = readPrivateE2BFile(linked, 1024); err == nil {
			t.Fatal("linked operator config was accepted")
		}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(private, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := readPrivateE2BFile(private, 1024); err == nil {
			t.Fatal("public operator config was accepted")
		}
	}
	good := "https://{sandboxID}.guests.example.test"
	if !validE2BOriginTemplate(good) {
		t.Fatal("private authenticated guest origin was refused")
	}
	for _, value := range []string{"http://{sandboxID}.guests.example.test", "https://{sandboxID}.guests.example.test:5008", "https://{sandboxID}.guests.example.test/x", "https://user@{sandboxID}.guests.example.test", "https://{sandboxID}.guests.example.test#fragment", "https://{sandboxID}.guests.example.test?x=1", strings.ReplaceAll(good, "{sandboxID}", "fixed")} {
		if validE2BOriginTemplate(value) {
			t.Fatalf("untrusted guest origin was accepted: %s", value)
		}
	}
}

func TestCloudRevocationRetainsOnlyExactOwnedObservationAndCleanupEffects(t *testing.T) {
	for _, action := range []string{"cancel", "observe", "guest-transport", "observe-launch", "observe-native-exit", "capture-output", "review-output"} {
		if !cloudOwnershipOnlyEffect(action) {
			t.Fatalf("prior owned attempt cannot be settled after revocation: %s", action)
		}
	}
	for _, action := range []string{"allocate", "stage-input", "start", "import-source", "", "deploy"} {
		if cloudOwnershipOnlyEffect(action) {
			t.Fatalf("new or unrecognized effect inherited cleanup authority: %s", action)
		}
	}
}
