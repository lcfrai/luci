package conformance

import (
	"bytes"
	"io"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// The designs group: server tasks 21 (blobs) and 22 (approvals, session
// links), contract §7.6–§7.8.
var designRows = []row{
	{name: "blob_hash_checked", fn: blobHashChecked},
	{name: "blob_type_allowlist", fn: blobTypeAllowlist},
	{name: "blob_too_large", fn: blobTooLarge},
	{name: "blob_other_project_404", fn: blobOtherProject404},
	{name: "blob_roles", fn: blobRoles},
	{name: "blob_quota", fn: blobQuota},
	{name: "put_blob_rewinds", fn: putBlobRewinds},
	{name: "approval_checks_in_order", fn: approvalChecksInOrder},
	{name: "approver_is_the_caller", fn: approverIsTheCaller},
	{name: "approval_of_an_older_revision", fn: approvalOfAnOlderRevision},
	{name: "approvals_list_and_get", fn: approvalsListAndGet},
	{name: "session_link_repeat_returns_existing", fn: sessionLinkRepeatReturnsExisting},
	{name: "session_link_checks", fn: sessionLinkChecks},
}

const htmlType = "text/html; charset=utf-8"

// fresh is bytes no other row uploads.
func fresh(label string) []byte { return []byte("<p>" + label + " " + uniq() + "</p>") }

func digestOf(b []byte) string { return hubapi.DigestOf(b).String() }

// put uploads data as ct to projectID through h.
func put(h hubclient.Hub, projectID, ct string, data []byte) (hubapi.Blob, error) {
	return h.PutBlob(bg, projectID, digestOf(data), ct, int64(len(data)), bytes.NewReader(data))
}

// upload is put, which must succeed and answer the blob as stored.
func upload(t *testing.T, h hubclient.Hub, projectID, ct string, data []byte) hubapi.Blob {
	t.Helper()
	b := must(put(h, projectID, ct, data))(t)
	if b.Digest.String() != digestOf(data) || b.Size != int64(len(data)) || b.ContentType != ct {
		t.Fatalf("PutBlob = %+v, want %s, %d bytes, %s", b, digestOf(data), len(data), ct)
	}
	return b
}

// read is GetBlob's bytes and blob, which must succeed.
func read(t *testing.T, h hubclient.Hub, projectID, digest string) ([]byte, hubapi.Blob) {
	t.Helper()
	rc, b, err := h.GetBlob(bg, projectID, digest)
	ok(t, err)
	defer rc.Close()
	data, err := io.ReadAll(rc)
	ok(t, err)
	return data, b
}

// noBlob asserts GetBlob of digest through projectID is 404 blob_not_found.
func noBlob(t *testing.T, h hubclient.Hub, projectID, digest string) {
	t.Helper()
	rc, _, err := h.GetBlob(bg, projectID, digest)
	if rc != nil {
		rc.Close()
	}
	wantErr(t, err, 404, hubapi.CodeBlobNotFound)
}

// bytes that do not hash to the digest → 422 digest_mismatch, nothing stored;
// the right bytes are stored and read back.
func blobHashChecked(t *testing.T, e Env) {
	p := newProject(t, e)
	data, other := fresh("page"), fresh("other")
	_, err := e.Member.PutBlob(bg, p.ID, digestOf(data), htmlType, int64(len(other)), bytes.NewReader(other))
	wantErr(t, err, 422, hubapi.CodeDigestMismatch)
	noBlob(t, e.Member, p.ID, digestOf(data))
	noBlob(t, e.Member, p.ID, digestOf(other))
	upload(t, e.Member, p.ID, htmlType, data)
	if got, b := read(t, e.Viewer, p.ID, digestOf(data)); !bytes.Equal(got, data) || b.ContentType != htmlType || b.Size != int64(len(data)) {
		t.Errorf("GetBlob = %q as %+v, want the bytes uploaded as %s", got, b, htmlType)
	}
}

// every allowlisted type is stored as sent; anything else → 400 Content-Type.
func blobTypeAllowlist(t *testing.T, e Env) {
	p := newProject(t, e)
	for _, ct := range hubapi.BlobContentTypes {
		upload(t, e.Member, p.ID, ct, fresh(ct))
	}
	for _, ct := range []string{"text/html", "application/javascript", "", "TEXT/HTML; charset=utf-8"} {
		data := fresh(ct)
		_, err := put(e.Member, p.ID, ct, data)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "Content-Type")
		noBlob(t, e.Member, p.ID, digestOf(data))
	}
}

// one byte over 16 MiB → 413 blob_too_large.
func blobTooLarge(t *testing.T, e Env) {
	p := newProject(t, e)
	data := make([]byte, hubapi.MaxBlobBytes+1)
	copy(data, uniq())
	_, err := put(e.Member, p.ID, "application/octet-stream", data)
	wantErr(t, err, 413, hubapi.CodeBlobTooLarge)
	noBlob(t, e.Member, p.ID, digestOf(data))
}

// a blob is read only through a project it is associated with; the same
// bytes uploaded to a second project associate there too.
func blobOtherProject404(t *testing.T, e Env) {
	p1, p2 := newProject(t, e), newProject(t, e)
	data := fresh("shared")
	upload(t, e.Member, p1.ID, htmlType, data)
	noBlob(t, e.Member, p2.ID, digestOf(data))
	// The first upload's type wins for its project: a re-upload answers the
	// stored type (task 21 step 6); another project's association has its own.
	const plain = "text/plain; charset=utf-8"
	if again := must(put(e.Member, p1.ID, plain, data))(t); again.ContentType != htmlType {
		t.Errorf("a re-upload as %s answered %s, want the stored %s", plain, again.ContentType, htmlType)
	}
	if _, b := read(t, e.Viewer, p1.ID, digestOf(data)); b.ContentType != htmlType {
		t.Errorf("after a re-upload p1 serves %s, want the first upload's %s", b.ContentType, htmlType)
	}
	upload(t, e.Member, p2.ID, plain, data)
	if got, b := read(t, e.Member, p2.ID, digestOf(data)); !bytes.Equal(got, data) || b.ContentType != plain {
		t.Errorf("p2's copy = %q as %s, want the bytes as %s", got, b.ContentType, plain)
	}
}

// member to put, viewer to get; a stranger is told there is no project.
func blobRoles(t *testing.T, e Env) {
	p := newProject(t, e)
	data := fresh("roles")
	_, err := put(e.Viewer, p.ID, htmlType, data)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	noBlob(t, e.Member, p.ID, digestOf(data))
	upload(t, e.Admin, p.ID, htmlType, data)
	if got, _ := read(t, e.Viewer, p.ID, digestOf(data)); !bytes.Equal(got, data) {
		t.Errorf("the viewer read %q, want the bytes", got)
	}
	for _, s := range strangers(t, e) {
		_, err := put(s.hub, p.ID, htmlType, fresh("stranger"))
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
		rc, _, err := s.hub.GetBlob(bg, p.ID, digestOf(data))
		if rc != nil {
			rc.Close()
		}
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", p.ID)
	}
}

// past the project's quota → 409 quota_exceeded; a re-upload counts nothing.
func blobQuota(t *testing.T, e Env) {
	if e.BlobQuota <= 0 {
		t.Skip("conformance: this Env names no BlobQuota; no row uploads a gibibyte to reach the default")
	}
	p, p2 := newProject(t, e), newProject(t, e)
	half := func() []byte { b := make([]byte, e.BlobQuota/2+1); copy(b, uniq()); return b }
	a, b := half(), half()
	upload(t, e.Member, p.ID, "application/octet-stream", a)
	_, err := put(e.Member, p.ID, "application/octet-stream", b)
	wantErr(t, err, 409, hubapi.CodeQuotaExceeded)
	noBlob(t, e.Member, p.ID, digestOf(b))
	upload(t, e.Member, p.ID, "application/octet-stream", a)
	upload(t, e.Member, p2.ID, "application/octet-stream", b)
}

// a reader already read to its end is rewound: the whole blob is stored (HA5-1).
func putBlobRewinds(t *testing.T, e Env) {
	p := newProject(t, e)
	data := fresh("rewound")
	r := bytes.NewReader(data)
	if _, err := io.ReadAll(r); err != nil {
		t.Fatal(err)
	}
	b := must(e.Member.PutBlob(bg, p.ID, digestOf(data), htmlType, int64(len(data)), r))(t)
	if b.Size != int64(len(data)) {
		t.Errorf("PutBlob of a consumed reader stored %d bytes, want %d", b.Size, len(data))
	}
	if got, _ := read(t, e.Member, p.ID, digestOf(data)); !bytes.Equal(got, data) {
		t.Errorf("read back %q, want %q", got, data)
	}
}

// approvable is what an approval needs: a project, a complete brief revision,
// an uploaded snapshot, and the request approving it under a fresh design key.
type approvable struct {
	p   hubapi.Project
	rev hubapi.BriefRevision
	req hubapi.ApprovalRequest
}

func newDesignKey() string { return "design:cv-" + uniq() + "@11" }

// approvableOn is an approvable for rev, a complete revision of a brief of p.
func approvableOn(t *testing.T, e Env, p hubapi.Project, rev hubapi.BriefRevision) approvable {
	t.Helper()
	manifest := upload(t, e.Member, p.ID, "application/json", []byte(`{"manifest":"`+uniq()+`"}`))
	page := upload(t, e.Member, p.ID, htmlType, fresh("artboard"))
	return approvable{p: p, rev: rev, req: hubapi.ApprovalRequest{DesignKey: newDesignKey(),
		ReviewDigest: hubapi.DigestOf(fresh("review")), BriefKey: rev.Key, BriefDigest: rev.Digest, BranchRef: featRef,
		Snapshot: hubapi.SnapshotRequest{Manifest: manifest.Digest, Blobs: []hubapi.Digest{manifest.Digest, page.Digest}}}}
}

// newApprovable is an approvable in a new project of T.
func newApprovable(t *testing.T, e Env) approvable {
	t.Helper()
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	return approvableOn(t, e, p, write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete))
}

// approve is a new approvable, approved by T's member.
func approve(t *testing.T, e Env) (approvable, hubapi.DesignApproval) {
	t.Helper()
	a := newApprovable(t, e)
	return a, must(e.Member.Approve(bg, a.p.ID, a.req))(t)
}

// the six checks of contract §7.7, in order: a body failing only check n, and
// a body failing check n and every later one, both answer check n's code.
func approvalChecksInOrder(t *testing.T, e Env) {
	a, first := approve(t, e)
	open := write(t, e.Member, a.p.ID, a.rev.BriefID, 1, hubapi.StatusOpen)
	absent := hubapi.DigestOf(fresh("never uploaded"))
	breaks := []func(r *hubapi.ApprovalRequest){
		1: func(r *hubapi.ApprovalRequest) { r.DesignKey = "design:X@1" },
		2: func(r *hubapi.ApprovalRequest) {
			r.BriefKey = hubapi.BriefKey{BriefID: a.rev.BriefID, Revision: 9}.String()
		},
		3: func(r *hubapi.ApprovalRequest) { r.BriefDigest = hubapi.DigestOf(fresh("wrong")) },
		4: func(r *hubapi.ApprovalRequest) { r.BriefKey, r.BriefDigest = open.Key, open.Digest },
		5: func(r *hubapi.ApprovalRequest) {
			r.Snapshot.Blobs = append(r.Snapshot.Blobs[:len(r.Snapshot.Blobs):len(r.Snapshot.Blobs)], absent)
		},
		6: func(r *hubapi.ApprovalRequest) { r.DesignKey = first.DesignKey },
	}
	want := func(t *testing.T, n int, r hubapi.ApprovalRequest, err error) {
		t.Helper()
		switch n {
		case 1:
			wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "designKey")
		case 2:
			wantErr(t, err, 422, hubapi.CodeKeyUnresolvable, "key", r.BriefKey)
		case 3:
			wantErr(t, err, 422, hubapi.CodeDigestMismatch, "key", r.BriefKey)
		case 4:
			wantErr(t, err, 422, hubapi.CodeBriefIncomplete, "key", r.BriefKey)
		case 5:
			wantErr(t, err, 422, hubapi.CodeBlobMissing, "digest", absent.String())
		case 6:
			wantErr(t, err, 409, hubapi.CodeDesignAlreadyApproved, "designKey", first.DesignKey, "approvalId", first.ID)
		}
	}
	base := a.req
	base.DesignKey = newDesignKey() // a's own key is approved: that is check 6
	for n := 1; n <= 6; n++ {
		only, all := base, base
		breaks[n](&only)
		for m := 6; m >= n; m-- {
			breaks[m](&all)
		}
		_, err := e.Member.Approve(bg, a.p.ID, only)
		t.Run("check_"+string(rune('0'+n))+"_alone", func(t *testing.T) { want(t, n, only, err) })
		_, err = e.Member.Approve(bg, a.p.ID, all)
		t.Run("check_"+string(rune('0'+n))+"_first", func(t *testing.T) { want(t, n, all, err) })
	}
	approvalCheck1And5Fields(t, e, a)
	if e.Erase == nil {
		t.Log("conformance: this Env has no Erase; check 2's erased revision is not run")
	} else { // check 2: a revision of this project's brief, but erased, approves nothing
		gone := newBrief(t, e.Member, a.p.ID, "refs/heads/gone")
		gr := write(t, e.Member, a.p.ID, gone.ID, 0, hubapi.StatusComplete)
		ok(t, e.Erase(gone.ID, "Conformance: an erased revision approves nothing"))
		r := base
		r.BriefKey, r.BriefDigest = gr.Key, gr.Digest
		_, err := e.Member.Approve(bg, a.p.ID, r)
		wantErr(t, err, 422, hubapi.CodeKeyUnresolvable, "key", gr.Key)
	}
	// check 5: a file another project of the org holds is still missing here.
	theirs := upload(t, e.Member, newProject(t, e).ID, htmlType, fresh("elsewhere"))
	r := base
	r.Snapshot.Blobs = append(r.Snapshot.Blobs[:len(r.Snapshot.Blobs):len(r.Snapshot.Blobs)], theirs.Digest)
	_, err := e.Member.Approve(bg, a.p.ID, r)
	wantErr(t, err, 422, hubapi.CodeBlobMissing, "digest", theirs.Digest.String())
	// check 6 is this project's: the same key approved in another project is 201.
	o := newApprovable(t, e)
	o.req.DesignKey = first.DesignKey
	if ap := must(e.Member.Approve(bg, o.p.ID, o.req))(t); ap.ProjectID != o.p.ID || ap.ID == first.ID {
		t.Errorf("the key approved in a second project = %+v, want a new approval there", ap)
	}
	if page := must(e.Viewer.Approvals(bg, a.p.ID, hubapi.ApprovalQuery{}))(t); len(page.Items) != 1 || page.Items[0].ID != first.ID {
		t.Errorf("after the refusals the project's approvals are %d, want only %s", len(page.Items), first.ID)
	}
}

// check 1's other fields, and check 5's 400s (count, duplicates, manifest).
func approvalCheck1And5Fields(t *testing.T, e Env, a approvable) {
	t.Helper()
	m, rest := a.req.Snapshot.Manifest, a.req.Snapshot.Blobs[1:]
	for field, mutate := range map[string]func(r *hubapi.ApprovalRequest){
		"briefKey":  func(r *hubapi.ApprovalRequest) { r.BriefKey = "brief:bf-x@1" },
		"branchRef": func(r *hubapi.ApprovalRequest) { r.BranchRef = "feat" },
		"snapshot.blobs": func(r *hubapi.ApprovalRequest) {
			r.Snapshot.Blobs = append([]hubapi.Digest{m, m}, rest...)
		},
	} {
		r := a.req
		r.DesignKey = newDesignKey()
		mutate(&r)
		_, err := e.Member.Approve(bg, a.p.ID, r)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", field)
	}
	for _, blobs := range [][]hubapi.Digest{{}, rest} { // none; the manifest absent
		r := a.req
		r.DesignKey, r.Snapshot.Blobs = newDesignKey(), blobs
		_, err := e.Member.Approve(bg, a.p.ID, r)
		wantErr(t, err, 400, hubapi.CodeBadRequest, "field", "snapshot.blobs")
	}
}

// approvedBy is the caller (the body has no field for it); the approval
// carries the key's parts and the snapshot as stored blobs, in request order.
func approverIsTheCaller(t *testing.T, e Env) {
	a := newApprovable(t, e)
	_, err := e.Viewer.Approve(bg, a.p.ID, a.req)
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	for _, s := range strangers(t, e) {
		_, err := s.hub.Approve(bg, a.p.ID, a.req)
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", a.p.ID)
	}
	ap := must(e.Member.Approve(bg, a.p.ID, a.req))(t)
	me := meOf(t, e.Member).User
	dk, _ := hubapi.ParseDesignKey(a.req.DesignKey)
	if ap.ApprovedBy.ID != me.ID || ap.ProjectID != a.p.ID || ap.DesignKey != a.req.DesignKey || ap.CanvasID != dk.CanvasID ||
		ap.DesignRevision != dk.Revision || ap.BriefKey != a.rev.Key || ap.BriefDigest != a.rev.Digest ||
		ap.ReviewDigest != a.req.ReviewDigest || ap.BranchRef != featRef || ap.ApprovedAt == "" || !hubapi.ValidID(hubapi.PrefixApproval, ap.ID) {
		t.Errorf("Approve = %+v, want the member's approval of %s", ap, a.req.DesignKey)
	}
	if len(ap.Snapshot.Blobs) != len(a.req.Snapshot.Blobs) || ap.Snapshot.Manifest != a.req.Snapshot.Manifest {
		t.Fatalf("snapshot = %+v, want the request's %d blobs", ap.Snapshot, len(a.req.Snapshot.Blobs))
	}
	for i, b := range ap.Snapshot.Blobs {
		if b.Digest != a.req.Snapshot.Blobs[i] || b.Size == 0 || b.ContentType == "" {
			t.Errorf("snapshot blob %d = %+v, want %s with its size and type", i, b, a.req.Snapshot.Blobs[i])
		}
	}
	if got := must(e.Viewer.Approval(bg, ap.ID))(t); got.ApprovedBy.ID != me.ID || got.ID != ap.ID {
		t.Errorf("Approval(%s) as the viewer = %+v, want the member as approver", ap.ID, got)
	}
}

// an approval against a complete revision older than the head is allowed.
func approvalOfAnOlderRevision(t *testing.T, e Env) {
	p := newProject(t, e)
	b := newBrief(t, e.Member, p.ID, featRef)
	r1 := write(t, e.Member, p.ID, b.ID, 0, hubapi.StatusComplete)
	write(t, e.Member, p.ID, b.ID, 1, hubapi.StatusComplete)
	a := approvableOn(t, e, p, r1)
	if ap := must(e.Member.Approve(bg, p.ID, a.req))(t); ap.BriefKey != r1.Key {
		t.Errorf("the approval names %s, want %s", ap.BriefKey, r1.Key)
	}
}

// Approvals filters by canvas and branch, newest first; Approval by id; an
// unknown id, or one the caller cannot read, is 404 approval_not_found.
func approvalsListAndGet(t *testing.T, e Env) {
	a, first := approve(t, e)
	tick(e)
	second := approvableOn(t, e, a.p, a.rev)
	second.req.BranchRef = "refs/heads/other"
	ap2 := must(e.Member.Approve(bg, a.p.ID, second.req))(t)
	ids := func(q hubapi.ApprovalQuery) []string {
		var out []string
		for _, ap := range must(e.Viewer.Approvals(bg, a.p.ID, q))(t).Items {
			out = append(out, ap.ID)
		}
		return out
	}
	c2, _ := hubapi.ParseDesignKey(ap2.DesignKey)
	for name, c := range map[string]struct {
		q    hubapi.ApprovalQuery
		want []string
	}{
		"all, newest first": {hubapi.ApprovalQuery{}, []string{ap2.ID, first.ID}},
		"canvasId":          {hubapi.ApprovalQuery{CanvasID: c2.CanvasID}, []string{ap2.ID}},
		"branchRef":         {hubapi.ApprovalQuery{BranchRef: featRef}, []string{first.ID}},
	} {
		if got := ids(c.q); len(got) != len(c.want) || (len(got) > 0 && got[0] != c.want[0]) || (len(got) > 1 && got[1] != c.want[1]) {
			t.Errorf("Approvals %s = %v, want %v", name, got, c.want)
		}
	}
	if got := must(e.Viewer.Approval(bg, first.ID))(t); got.DesignKey != first.DesignKey {
		t.Errorf("Approval(%s) = %+v", first.ID, got)
	}
	_, err := e.Viewer.Approval(bg, "ap-0000000000000000")
	wantErr(t, err, 404, hubapi.CodeApprovalNotFound)
	for _, s := range strangers(t, e) {
		_, err := s.hub.Approval(bg, first.ID)
		wantErr(t, err, 404, hubapi.CodeApprovalNotFound)
		_, err = s.hub.Approvals(bg, a.p.ID, hubapi.ApprovalQuery{})
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", a.p.ID)
	}
}

func newSessionID() string { return "s-" + uniq()[:8] }

// linkOf is a session link request for ap under a fresh session id.
func linkOf(ap hubapi.DesignApproval) hubapi.SessionLinkRequest {
	return hubapi.SessionLinkRequest{DesignKey: ap.DesignKey, BriefKey: ap.BriefKey, SessionID: newSessionID(), BranchRef: featRef}
}

// the same user repeating (designKey, sessionId) gets the same link back;
// another user making the same pair makes a second link.
func sessionLinkRepeatReturnsExisting(t *testing.T, e Env) {
	a, ap := approve(t, e)
	req := linkOf(ap)
	l1 := must(e.Member.LinkSession(bg, a.p.ID, req))(t)
	if l1.CreatedBy.ID != meOf(t, e.Member).User.ID || !hubapi.ValidID(hubapi.PrefixLink, l1.ID) || l1.SessionID != req.SessionID {
		t.Errorf("LinkSession = %+v, want the member's ln- link", l1)
	}
	if again := must(e.Member.LinkSession(bg, a.p.ID, req))(t); again.ID != l1.ID {
		t.Errorf("a repeat made link %s, want the existing %s", again.ID, l1.ID)
	}
	tick(e)
	l2 := must(e.Admin.LinkSession(bg, a.p.ID, req))(t)
	if l2.ID == l1.ID {
		t.Errorf("another user's link reused %s", l1.ID)
	}
	page := must(e.Viewer.SessionLinks(bg, a.p.ID, hubapi.SessionLinkQuery{DesignKey: ap.DesignKey, SessionID: req.SessionID}))(t)
	if len(page.Items) != 2 || page.Items[0].ID != l2.ID || page.Items[1].ID != l1.ID {
		t.Errorf("SessionLinks = %+v, want %s then %s, newest first", page.Items, l2.ID, l1.ID)
	}
}

// §7.8's refusals: design_not_approved, key_unresolvable, the grammar, the
// roles; an erased brief key still links (task 22: the stamped key exists).
func sessionLinkChecks(t *testing.T, e Env) {
	a, ap := approve(t, e)
	other := newProject(t, e)
	elsewhere := write(t, e.Member, other.ID, newBrief(t, e.Member, other.ID, featRef).ID, 0, hubapi.StatusOpen)
	cases := []struct {
		mutate func(r *hubapi.SessionLinkRequest)
		status int
		code   string
		kv     []string
	}{
		{func(r *hubapi.SessionLinkRequest) { r.DesignKey = "design:cv-0000@1" }, 422, hubapi.CodeDesignNotApproved, []string{"designKey", "design:cv-0000@1"}},
		{func(r *hubapi.SessionLinkRequest) { r.BriefKey = elsewhere.Key }, 422, hubapi.CodeKeyUnresolvable, []string{"key", elsewhere.Key}},
		{func(r *hubapi.SessionLinkRequest) {
			r.BriefKey = hubapi.BriefKey{BriefID: a.rev.BriefID, Revision: 9}.String()
		}, 422, hubapi.CodeKeyUnresolvable, nil},
		{func(r *hubapi.SessionLinkRequest) { r.SessionID = "S-1" }, 400, hubapi.CodeBadRequest, []string{"field", "sessionId"}},
		{func(r *hubapi.SessionLinkRequest) { r.DesignKey = "design:X@1" }, 400, hubapi.CodeBadRequest, []string{"field", "designKey"}},
		{func(r *hubapi.SessionLinkRequest) { r.BranchRef = "feat" }, 400, hubapi.CodeBadRequest, []string{"field", "branchRef"}},
	}
	for _, c := range cases {
		r := linkOf(ap)
		c.mutate(&r)
		_, err := e.Member.LinkSession(bg, a.p.ID, r)
		wantErr(t, err, c.status, c.code, c.kv...)
	}
	_, err := e.Member.LinkSession(bg, other.ID, linkOf(ap)) // approved, but in another project
	wantErr(t, err, 422, hubapi.CodeDesignNotApproved, "designKey", ap.DesignKey)
	_, err = e.Viewer.LinkSession(bg, a.p.ID, linkOf(ap))
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "member")
	for _, s := range strangers(t, e) {
		_, err := s.hub.LinkSession(bg, a.p.ID, linkOf(ap))
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", a.p.ID)
		_, err = s.hub.SessionLinks(bg, a.p.ID, hubapi.SessionLinkQuery{})
		wantErr(t, err, 404, hubapi.CodeProjectNotFound, "projectId", a.p.ID)
	}
	if page := must(e.Viewer.SessionLinks(bg, a.p.ID, hubapi.SessionLinkQuery{}))(t); len(page.Items) != 0 || page.Items == nil {
		t.Errorf("after the refusals the project has %d links, want []", len(page.Items))
	}
	if e.Erase == nil {
		t.Log("conformance: this Env has no Erase; the erased brief key's link is not run")
		return
	}
	ok(t, e.Erase(a.rev.BriefID, "Conformance: a stamped key outlives its content"))
	if l := must(e.Member.LinkSession(bg, a.p.ID, linkOf(ap)))(t); l.BriefKey != a.rev.Key {
		t.Errorf("a link to an erased revision = %+v, want its key %s", l, a.rev.Key)
	}
}
