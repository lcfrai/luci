package product

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateCloudReleaseStopObservesProviderAndRollbackPinsHistory(t *testing.T) {
	provider := &cloudProviderFixture{status: "finished", appStatus: "running:healthy"}
	s, mux, p, req, remote := cloudReleaseFixture(t, provider)
	req.Source = "private"
	req.PrivateKeyUUID = "key-owned"
	base := "/v1/product/projects/" + p.ID + "/releases"
	first := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	if provider.privateRepository != "git@github.com:example/project.git" || provider.privateKey != "key-owned" || first.Repository != req.Repository {
		t.Fatal("private source mapping lost HTTPS provenance or deploy-key ownership")
	}
	first.CoordinatorOriginDigest = strings.Repeat("c", 64)
	first.Status = "ready"
	if err := s.storeCloudRelease(first); err != nil {
		t.Fatal(err)
	}
	stopping := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+first.ID+"/stop", nil, 202))
	if stopping.Status != "stopping" || stopping.URL != "" {
		t.Fatal("queued stop was treated as confirmed")
	}
	pending := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+first.ID+"/refresh", nil, 200))
	if pending.Status != "stopping" {
		t.Fatal("running provider was treated as stopped")
	}
	journeyRequest(t, mux, "POST", base+"/"+first.ID+"/stop", nil, 200)
	if provider.stops != 1 {
		t.Fatal("repeated stop queued a duplicate mutation")
	}
	provider.appStatus = "exited:unhealthy"
	stopped := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+first.ID+"/refresh", nil, 200))
	if stopped.Status != "stopped" {
		t.Fatal("actual stopped state was not observed")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "next.txt"), []byte("next version"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	commit, _ := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	req.Commit = strings.TrimSpace(commit)
	second := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/cloud", req, 202))
	second.Status = "ready"
	_ = s.storeCloudRelease(second)
	// A historical stop must not stop the newer commit on the same application.
	// Reset first's historical state to ready to exercise ownership rather than
	// the idempotent stopped-record response.
	first.Status = "ready"
	_ = s.storeCloudRelease(first)
	journeyRequest(t, mux, "POST", base+"/"+first.ID+"/stop", nil, 409)
	rolled := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+first.ID+"/rollback", map[string]any{}, 202))
	if rolled.CoordinatorOriginDigest != "" || rolled.ID == first.ID || rolled.RollbackOf != first.ID || rolled.Release.Commit != first.Release.Commit || rolled.BriefRevision != first.BriefRevision || rolled.ApplicationID != first.ApplicationID || rolled.Status != "queued" || rolled.URL != "" {
		t.Fatalf("rollback lost historical provenance: %+v", rolled)
	}
	// The separate recovery action must reach the primary conversation through
	// retained release history, without rewriting the original delivery.
	activity, err := s.orchestratorActivityLocked(p, "alice")
	if err != nil || len(activity.Recovery) != 1 || activity.Recovery[0].ReleaseID != rolled.ID || activity.Recovery[0].RollbackOf != first.ID || activity.Recovery[0].Status != "queued" {
		t.Fatalf("queued recovery was not visible by exact identity: %+v, %v", activity, err)
	}
	shared, err := s.orchestratorActivityLocked(p, "")
	if err != nil || len(shared.Recovery) != 0 {
		t.Fatalf("project summary exposed per-person release recovery: %+v, %v", shared, err)
	}
	s.mu.RLock()
	retained, err := s.readCloudReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		t.Fatal(err)
	}
	preserved := false
	for _, row := range retained {
		if row.ID == first.ID {
			preserved = row.CoordinatorOriginDigest == first.CoordinatorOriginDigest
		}
	}
	if !preserved {
		t.Fatal("manual rebuild changed the original coordinator receipt")
	}
	count := provider.deployments
	repeated := decodeCloudRelease(t, journeyRequest(t, mux, "POST", base+"/"+first.ID+"/rollback", map[string]any{}, 202))
	if repeated.ID != rolled.ID || provider.deployments != count {
		t.Fatal("retry duplicated historical rebuild")
	}
	client, _ := NewCoolifyClient(remote.URL, testCoolifyToken)
	verified, err := observeCloudRelease(context.Background(), rolled, client, func(context.Context, string) error { return nil })
	if err != nil || verified.Status != "ready" || verified.Release.Commit != first.Release.Commit {
		t.Fatal("rollback could not verify its exact commit and health", err)
	}
	if err := s.storeCloudRelease(verified); err != nil {
		t.Fatal(err)
	}
	activity, err = s.orchestratorActivityLocked(p, "alice")
	if err != nil || len(activity.Recovery) != 1 || activity.Recovery[0].Status != "ready" || activity.Recovery[0].Commit != first.Release.Commit {
		t.Fatalf("observed recovery health did not survive durable reread: %+v, %v", activity, err)
	}
}

func TestPrivateApplicationPayloadRejectsNonGitHubAndMissingKeys(t *testing.T) {
	app := CoolifyGitApplication{ProjectUUID: "p", ServerUUID: "s", Name: "test", Repository: "https://github.com/owner/repo.git", Branch: "main", Commit: strings.Repeat("a", 40), Port: 3000, Domain: "https://app.example.com", Source: "private"}
	if _, err := coolifyGitApplicationPayload(app); err == nil {
		t.Fatal("private source accepted no key")
	}
	app.PrivateKeyUUID = "key-owned"
	app.Repository = "https://example.com/owner/repo.git"
	if _, err := coolifyGitApplicationPayload(app); err == nil {
		t.Fatal("unsupported host accepted")
	}
}
