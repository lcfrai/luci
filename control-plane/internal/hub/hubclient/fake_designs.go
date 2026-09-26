package hubclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// The fake's blobs, design approvals and session links (contract §7.6–§7.8;
// server tasks 21 and 22), and the design half of the resolver (§7.5).

// fakeBlobUse is one blob's association with one project (task 21's
// project_blobs row): the first upload's content type wins.
type fakeBlobUse struct {
	size        int64
	contentType string
}

type fakeApproval struct {
	id, projectID  string
	designKey      string
	canvasID       string
	designRevision int
	reviewDigest   hubapi.Digest
	briefKey       string
	briefDigest    hubapi.Digest
	branchRef      string
	manifest       hubapi.Digest
	blobs          []hubapi.Blob // the snapshot, in request order
	approvedAt     string
	approvedBy     string // a user id
	seq            int64
}

type fakeLink struct {
	id, projectID                             string
	designKey, briefKey, sessionID, branchRef string
	createdAt                                 string
	createdBy                                 string // a user id
	seq                                       int64
}

func (a *fakeApproval) order() int64 { return a.seq }
func (l *fakeLink) order() int64     { return l.seq }

func (w *fakeWorld) renderApproval(a *fakeApproval) hubapi.DesignApproval {
	return hubapi.DesignApproval{ID: a.id, ProjectID: a.projectID, DesignKey: a.designKey, CanvasID: a.canvasID,
		DesignRevision: a.designRevision, ReviewDigest: a.reviewDigest, BriefKey: a.briefKey, BriefDigest: a.briefDigest,
		BranchRef: a.branchRef, Snapshot: hubapi.Snapshot{Manifest: a.manifest, Blobs: append([]hubapi.Blob{}, a.blobs...)},
		ApprovedAt: a.approvedAt, ApprovedBy: w.users[a.approvedBy]}
}

func (w *fakeWorld) renderLink(l *fakeLink) hubapi.SessionLink {
	return hubapi.SessionLink{ID: l.id, ProjectID: l.projectID, DesignKey: l.designKey, BriefKey: l.briefKey,
		SessionID: l.sessionID, BranchRef: l.branchRef, CreatedAt: l.createdAt, CreatedBy: w.users[l.createdBy]}
}

// approvalIn is projectID's approval of designKey, or nil (§7.7 check 6).
func (w *fakeWorld) approvalIn(projectID, designKey string) *fakeApproval {
	for _, a := range w.approvals {
		if a.projectID == projectID && a.designKey == designKey {
			return a
		}
	}
	return nil
}

// revisionIn is the revision briefKey names in projectID, or nil: a brief of
// that project, and a revision it has.
func (w *fakeWorld) revisionIn(projectID, briefKey string) (*fakeBrief, *fakeRevision) {
	k, err := hubapi.ParseBriefKey(briefKey)
	if err != nil {
		return nil, nil
	}
	b := w.briefs[k.BriefID]
	if b == nil || b.projectID != projectID || k.Revision > len(b.revisions) {
		return nil, nil
	}
	return b, b.revisions[k.Revision-1]
}

// headInfo is b's entry in the resolver's heads.
func headInfo(b *fakeBrief) hubapi.HeadInfo {
	return hubapi.HeadInfo{HeadRevision: len(b.revisions), LatestCompleteRevision: b.latestComplete, Archived: b.archivedAt != ""}
}

// ── blobs (§7.6, task 21) ───────────────────────────────────────────────────

// PutBlob is route 28, in task 21's order: the digest's form, the content
// type, the size (413 before a byte is read), then the body — rewound to 0
// first, as the transport rewinds it before every attempt (HA5-1) — read and
// hashed (422, nothing stored); only then member of the project and the quota.
// Bytes are stored once per org and associated per project; the first
// upload's content type wins.
func (f *Fake) PutBlob(_ context.Context, projectID, digest, contentType string, size int64, body io.ReadSeeker) (hubapi.Blob, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("PutBlob", projectID, digest, contentType, size, "…")
	if err != nil {
		return hubapi.Blob{}, err
	}
	d, err := hubapi.ParseDigest(digest)
	if err != nil {
		return hubapi.Blob{}, badRequest("sha", "A blob is named by sha256: and 64 lowercase hex characters.")
	}
	if !slices.Contains(hubapi.BlobContentTypes, contentType) {
		return hubapi.Blob{}, badRequest("Content-Type", "That content type is not one the Hub stores.")
	}
	switch {
	case size < 0:
		return hubapi.Blob{}, badRequest("Content-Length", "A blob needs its length.")
	case size > hubapi.MaxBlobBytes:
		return hubapi.Blob{}, problem(413, hubapi.CodeBlobTooLarge, "A blob is at most 16 MiB.", "limitBytes", strconv.Itoa(hubapi.MaxBlobBytes))
	}
	var data []byte
	if body != nil {
		if _, err := body.Seek(0, io.SeekStart); err != nil {
			return hubapi.Blob{}, fmt.Errorf("hubclient: rewinding the blob body: %w", err)
		}
		if data, err = io.ReadAll(io.LimitReader(body, size+1)); err != nil {
			return hubapi.Blob{}, fmt.Errorf("hubclient: reading the blob body: %w", err)
		}
	}
	if int64(len(data)) != size {
		return hubapi.Blob{}, badRequest("Content-Length", "The body is not the length it declared.")
	}
	if hubapi.DigestOf(data) != d {
		return hubapi.Blob{}, problem(422, hubapi.CodeDigestMismatch, "Those bytes do not hash to the digest they were sent under.", "digest", digest)
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Blob{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.Blob{}, err
	}
	uses := w.uses[p.id]
	if u := uses[d]; u != nil {
		return hubapi.Blob{Digest: d, Size: u.size, ContentType: u.contentType}, nil // 200: already this project's
	}
	var total int64
	for _, u := range uses {
		total += u.size
	}
	if total+size > w.quota {
		return hubapi.Blob{}, problem(409, hubapi.CodeQuotaExceeded, "This project's stored files are at their quota.", "limitBytes", strconv.FormatInt(w.quota, 10))
	}
	if w.blobs[d] == nil {
		w.blobs[d] = slices.Clone(data)
	}
	if uses == nil {
		uses = map[hubapi.Digest]*fakeBlobUse{}
		w.uses[p.id] = uses
	}
	uses[d] = &fakeBlobUse{size: size, contentType: contentType}
	w.emitIn(me, "blob.uploaded", p, d.String(), "size", int(size), "contentType", contentType) // audit only
	return hubapi.Blob{Digest: d, Size: size, ContentType: contentType}, nil
}

// GetBlob is route 29: viewer of the project; a blob not associated with this
// project is 404 blob_not_found, whichever other project holds it.
func (f *Fake) GetBlob(_ context.Context, projectID, digest string) (io.ReadCloser, hubapi.Blob, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("GetBlob", projectID, digest)
	if err != nil {
		return nil, hubapi.Blob{}, err
	}
	d, err := hubapi.ParseDigest(digest)
	if err != nil {
		return nil, hubapi.Blob{}, badRequest("sha", "A blob is named by sha256: and 64 lowercase hex characters.")
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return nil, hubapi.Blob{}, err
	}
	u := w.uses[p.id][d]
	if u == nil {
		return nil, hubapi.Blob{}, problem(404, hubapi.CodeBlobNotFound, "This project holds no such file.", "digest", digest)
	}
	return io.NopCloser(bytes.NewReader(slices.Clone(w.blobs[d]))), hubapi.Blob{Digest: d, Size: u.size, ContentType: u.contentType}, nil
}

// ── design approvals (§7.7, task 22 route 30) ───────────────────────────────

// Approve is route 30: member of the project, then task 22's checks 1–6 in
// order, each with its error. The approver is the caller; the body has no
// field for it. An approval against a complete revision older than the head
// is allowed.
func (f *Fake) Approve(_ context.Context, projectID string, req hubapi.ApprovalRequest) (hubapi.DesignApproval, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Approve", projectID, req)
	if err != nil {
		return hubapi.DesignApproval{}, err
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.DesignApproval{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.DesignApproval{}, err
	}
	// 1. the grammar.
	dk, err := hubapi.ParseDesignKey(req.DesignKey)
	if err != nil {
		return hubapi.DesignApproval{}, badRequest("designKey", "A design key is design:<canvas id>@<revision>.")
	}
	if _, err := hubapi.ParseBriefKey(req.BriefKey); err != nil {
		return hubapi.DesignApproval{}, badRequest("briefKey", "A brief key is brief:<brief id>@<revision>.")
	}
	if req.BranchRef != "" && !hubapi.ValidBranchRef(req.BranchRef) {
		return hubapi.DesignApproval{}, badBranchRef()
	}
	// 2–4. the brief revision: this project's, not erased, this digest, complete.
	_, rev := w.revisionIn(p.id, req.BriefKey)
	if rev == nil || rev.erasedAt != "" {
		return hubapi.DesignApproval{}, problem(422, hubapi.CodeKeyUnresolvable, "That brief revision is not one of this project's.", "key", req.BriefKey)
	}
	if rev.digest != req.BriefDigest {
		return hubapi.DesignApproval{}, problem(422, hubapi.CodeDigestMismatch, "That digest is not the brief revision's.", "key", req.BriefKey)
	}
	if rev.status != hubapi.StatusComplete {
		return hubapi.DesignApproval{}, problem(422, hubapi.CodeBriefIncomplete, "A design is approved against a complete brief revision.", "key", req.BriefKey)
	}
	// 5. the snapshot: 1–500 distinct digests, the manifest among them, each this project's.
	blobs := req.Snapshot.Blobs
	if n := len(blobs); n < 1 || n > 500 || len(slices.Compact(slices.SortedFunc(slices.Values(blobs), compareDigest))) != n ||
		!slices.Contains(blobs, req.Snapshot.Manifest) {
		return hubapi.DesignApproval{}, badRequest("snapshot.blobs", "A snapshot is 1 to 500 distinct files, its manifest among them.")
	}
	stored := make([]hubapi.Blob, len(blobs))
	for i, d := range blobs {
		u := w.uses[p.id][d]
		if u == nil {
			return hubapi.DesignApproval{}, problem(422, hubapi.CodeBlobMissing, "A snapshot file is not uploaded to this project.", "digest", d.String())
		}
		stored[i] = hubapi.Blob{Digest: d, Size: u.size, ContentType: u.contentType}
	}
	// 6. one approval per design key per project.
	if a := w.approvalIn(p.id, req.DesignKey); a != nil {
		return hubapi.DesignApproval{}, problem(409, hubapi.CodeDesignAlreadyApproved, "That design revision is already approved.",
			"designKey", req.DesignKey, "approvalId", a.id)
	}
	a := &fakeApproval{id: ids.Wide(hubapi.PrefixApproval), projectID: p.id, designKey: req.DesignKey, canvasID: dk.CanvasID,
		designRevision: dk.Revision, reviewDigest: req.ReviewDigest, briefKey: req.BriefKey, briefDigest: req.BriefDigest,
		branchRef: req.BranchRef, manifest: req.Snapshot.Manifest, blobs: stored, approvedAt: w.stamp(), approvedBy: me, seq: w.next()}
	w.approvals[a.id] = a
	w.emitIn(me, "design.approved", p, a.id, "designKey", a.designKey, "briefKey", a.briefKey)
	return w.renderApproval(a), nil
}

func compareDigest(a, b hubapi.Digest) int { return bytes.Compare(a[:], b[:]) }

// newestFirst is rows of m in projectID newer-to-older, those before the
// cursor after when it names one of them (else 400 after), kept by keep.
func newestFirst[T interface {
	ordered
	comparable
}](m map[string]T, projectOf func(T) string, projectID, after string, keep func(T) bool) ([]T, error) {
	var from int64 = 1 << 62
	if after != "" {
		c, found := m[after]
		if !found || projectOf(c) != projectID {
			return nil, badRequest("after", "That cursor is not one this list gave.")
		}
		from = c.order()
	}
	var rows []T
	for _, r := range slices.Backward(inOrder(m)) {
		if projectOf(r) == projectID && r.order() < from && keep(r) {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

// Approval is route 32: unknown, unreadable and malformed are one 404.
func (f *Fake) Approval(_ context.Context, approvalID string) (hubapi.DesignApproval, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Approval", approvalID)
	if err != nil {
		return hubapi.DesignApproval{}, err
	}
	a := w.approvals[approvalID]
	if a == nil || !w.canRead(w.projects[a.projectID], me) {
		return hubapi.DesignApproval{}, problem(404, hubapi.CodeApprovalNotFound, "There is no such approval that you can see.", "approvalId", approvalID)
	}
	return w.renderApproval(a), nil
}

// Approvals is route 31: the project's approvals, newest first, filtered by
// canvas id and branch ref; the cursor is the last approval's id.
func (f *Fake) Approvals(_ context.Context, projectID string, q hubapi.ApprovalQuery) (hubapi.Page[hubapi.DesignApproval], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("Approvals", projectID, q)
	if err != nil {
		return hubapi.Page[hubapi.DesignApproval]{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Page[hubapi.DesignApproval]{}, err
	}
	if q.CanvasID != "" && !hubapi.ValidCanvasID(q.CanvasID) {
		return hubapi.Page[hubapi.DesignApproval]{}, badRequest("canvasId", "A canvas id is 2–4 letters, a hyphen and 4–32 hex.")
	}
	if q.BranchRef != "" && !hubapi.ValidBranchRef(q.BranchRef) {
		return hubapi.Page[hubapi.DesignApproval]{}, badBranchRef()
	}
	limit, err := pageLimit(q.Page.Limit)
	if err != nil {
		return hubapi.Page[hubapi.DesignApproval]{}, err
	}
	rows, err := newestFirst(w.approvals, func(a *fakeApproval) string { return a.projectID }, p.id, q.Page.After, func(a *fakeApproval) bool {
		return (q.CanvasID == "" || a.canvasID == q.CanvasID) && (q.BranchRef == "" || a.branchRef == q.BranchRef)
	})
	if err != nil {
		return hubapi.Page[hubapi.DesignApproval]{}, err
	}
	return pageOf(rows, limit, func(a *fakeApproval) string { return a.id }, w.renderApproval), nil
}

// ── session links (§7.8, task 22 routes 33–34) ──────────────────────────────

// LinkSession is route 33: member; the grammar; the design approved in this
// project (422 design_not_approved); the brief key a revision of this
// project's, erased or not (422 key_unresolvable: it is the key the control
// plane stamped, and it exists); a repeat of (designKey, sessionId) by the
// same user answers the existing link.
func (f *Fake) LinkSession(_ context.Context, projectID string, req hubapi.SessionLinkRequest) (hubapi.SessionLink, error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("LinkSession", projectID, req)
	if err != nil {
		return hubapi.SessionLink{}, err
	}
	p, mine, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.SessionLink{}, err
	}
	if err := need(mine, hubapi.RoleMember); err != nil {
		return hubapi.SessionLink{}, err
	}
	switch _, derr := hubapi.ParseDesignKey(req.DesignKey); {
	case derr != nil:
		return hubapi.SessionLink{}, badRequest("designKey", "A design key is design:<canvas id>@<revision>.")
	case second(hubapi.ParseBriefKey(req.BriefKey)) != nil:
		return hubapi.SessionLink{}, badRequest("briefKey", "A brief key is brief:<brief id>@<revision>.")
	case !hubapi.ValidSessionID(req.SessionID):
		return hubapi.SessionLink{}, badRequest("sessionId", "A session id is 1–4 letters, a hyphen and 8–32 hex.")
	case req.BranchRef != "" && !hubapi.ValidBranchRef(req.BranchRef):
		return hubapi.SessionLink{}, badBranchRef()
	}
	if w.approvalIn(p.id, req.DesignKey) == nil {
		return hubapi.SessionLink{}, problem(422, hubapi.CodeDesignNotApproved, "That design revision is not approved in this project.", "designKey", req.DesignKey)
	}
	if _, rev := w.revisionIn(p.id, req.BriefKey); rev == nil {
		return hubapi.SessionLink{}, problem(422, hubapi.CodeKeyUnresolvable, "That brief revision is not one of this project's.", "key", req.BriefKey)
	}
	for _, l := range w.links {
		if l.projectID == p.id && l.designKey == req.DesignKey && l.sessionID == req.SessionID && l.createdBy == me {
			return w.renderLink(l), nil // 200: the existing link
		}
	}
	l := &fakeLink{id: ids.Wide(hubapi.PrefixLink), projectID: p.id, designKey: req.DesignKey, briefKey: req.BriefKey,
		sessionID: req.SessionID, branchRef: req.BranchRef, createdAt: w.stamp(), createdBy: me, seq: w.next()}
	w.links[l.id] = l
	w.emitIn(me, "session_link.created", p, l.id, "designKey", l.designKey, "briefKey", l.briefKey, "sessionId", l.sessionID)
	return w.renderLink(l), nil
}

// second is a call's error alone.
func second[T any](_ T, err error) error { return err }

// SessionLinks is route 34: newest first, filtered by design key and session id.
func (f *Fake) SessionLinks(_ context.Context, projectID string, q hubapi.SessionLinkQuery) (hubapi.Page[hubapi.SessionLink], error) {
	w := f.w
	w.mu.Lock()
	defer w.mu.Unlock()
	me, err := f.enter("SessionLinks", projectID, q)
	if err != nil {
		return hubapi.Page[hubapi.SessionLink]{}, err
	}
	p, _, err := w.projectOf(projectID, me)
	if err != nil {
		return hubapi.Page[hubapi.SessionLink]{}, err
	}
	limit, err := pageLimit(q.Page.Limit)
	if err != nil {
		return hubapi.Page[hubapi.SessionLink]{}, err
	}
	rows, err := newestFirst(w.links, func(l *fakeLink) string { return l.projectID }, p.id, q.Page.After, func(l *fakeLink) bool {
		return (q.DesignKey == "" || l.designKey == q.DesignKey) && (q.SessionID == "" || l.sessionID == q.SessionID)
	})
	if err != nil {
		return hubapi.Page[hubapi.SessionLink]{}, err
	}
	return pageOf(rows, limit, func(l *fakeLink) string { return l.id }, w.renderLink), nil
}
