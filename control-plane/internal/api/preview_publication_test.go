package api

import (
	"context"
	"strings"
	"testing"
)

func TestImagePublicationLeaseKeepsOriginalCapacityAndCannotRepeatPublishing(t *testing.T) {
	g := previewTestGate(t)
	p := previewTestPrepared(g, "p-one", "preview-build-publish", "preview-build", 512, 1000)
	v := reservePreviewTest(t, g, p)
	bound, err := g.bindPreviewPublication(context.Background(), v, "registry.example/team/app", strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = g.bindPreviewPublication(context.Background(), v, "registry.example/other/app", strings.Repeat("b", 64)); err == nil {
		t.Fatal("stale original registry binding replaced")
	}
	v = bound
	v, err = g.transitionPreview(context.Background(), v, "building", "", "")
	if err != nil {
		t.Fatal(err)
	}
	image := "sha256:" + strings.Repeat("f", 64)
	v, err = g.transitionPreview(context.Background(), v, "built", "", image)
	if err != nil {
		t.Fatal(err)
	}
	if v.State == "finished" {
		t.Fatal("build released capacity before push")
	}
	v, err = g.transitionPreview(context.Background(), v, "publishing", "", image)
	if err != nil {
		t.Fatal(err)
	}
	reopened := newExecutionAdmission(g.api)
	read, err := reopened.previewLease(context.Background(), v.ID)
	if err != nil || read != v {
		t.Fatal("original marker did not survive reopen", err)
	}
	if _, err = reopened.transitionPreview(context.Background(), read, "publishing", "", image); err == nil {
		t.Fatal("unknown push was replayable")
	}
	if _, err = reopened.completePreview(context.Background(), read, false, image); err == nil {
		t.Fatal("unknown push released capacity")
	}
	settled, err := reopened.finishPreviewPublication(context.Background(), read, "sha256:"+strings.Repeat("c", 64))
	if err != nil || settled.State != "finished" || settled.PublicationDigest == "" {
		t.Fatal(settled, err)
	}
	if _, err = reopened.finishPreviewPublication(context.Background(), read, "sha256:"+strings.Repeat("d", 64)); err == nil {
		t.Fatal("late original settlement replaced retained digest")
	}
}
func TestImagePublicationLegacyBuildLeaseEncodingAndServingStateStaySeparate(t *testing.T) {
	g := previewTestGate(t)
	legacy := reservePreviewTest(t, g, previewTestPrepared(g, "p-one", "preview-build-old", "preview-build", 512, 1000))
	if !validPreviewExecutionLease(legacy) || legacy.PublicationRepository != "" {
		t.Fatal("legacy build lease changed")
	}
	changed := legacy
	changed.State = "built"
	changed.Image = "sha256:" + strings.Repeat("a", 64)
	if validPreviewExecutionLease(changed) {
		t.Fatal("unbound build imported publication phase")
	}
	changed = legacy
	changed.PublicationDigest = "sha256:" + strings.Repeat("b", 64)
	if validPreviewExecutionLease(changed) {
		t.Fatal("digest without original registry binding")
	}
}
