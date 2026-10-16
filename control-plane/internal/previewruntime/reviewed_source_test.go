package previewruntime

import "testing"

func TestSourceIdentityIsAdditiveAndCannotChangeKindSnapshotOrAcquireBuild(t *testing.T) {
	legacy := Identity{ProjectID: "project", ReleaseID: "release", Target: "local", Commit: "commit", SourceDigest: "source"}
	if !Continues(legacy, legacy) {
		t.Fatal("legacy identity stopped continuing")
	}
	source := legacy
	source.Kind = ReviewedSourceKind
	if Continues(legacy, source) || Continues(source, legacy) {
		t.Fatal("source and legacy image identities became interchangeable")
	}
	prepared := source
	prepared.SnapshotDigest = "snapshot"
	prepared.AllocationID = "allocation"
	if !Continues(source, prepared) {
		t.Fatal("first exact snapshot could not be acknowledged")
	}
	changed := prepared
	changed.SnapshotDigest = "changed"
	if Continues(prepared, changed) {
		t.Fatal("known full snapshot was replaceable")
	}
	changed = prepared
	changed.BuildAllocationID = "invented-build"
	if Continues(prepared, changed) {
		t.Fatal("source serving acquired an image-build lineage")
	}
	changed = legacy
	changed.SnapshotDigest = "snapshot"
	if Continues(legacy, changed) {
		t.Fatal("legacy identity silently acquired a static snapshot")
	}
}
