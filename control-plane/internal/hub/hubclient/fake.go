package hubclient

import (
	"cmp"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/ids"
)

// Fake is an in-memory Hub: one org ("Example Org", tenant
// 00000000-0000-0000-0000-00000000000e), one principal at a time. It is safe for
// concurrent use; every view (As) shares the same state.
//
// It keeps every rule of contract §7 a caller can observe — task 04: identity,
// teams, invites, projects, briefs, revisions and the resolver's brief half;
// task 05: the rest — and refuses as the Hub does: an *Error with the Hub's
// status, code and details. It is not the Hub. It has no Postgres, no
// row-level security, no audit chain, no idempotency store and no HTTP; the
// conformance suite (task 05) is what holds the two to one standard.
type Fake struct {
	w  *fakeWorld
	as FakeUser
}

// FakeUser names a principal. NewFake starts as FakeAda.
type FakeUser struct {
	ID, DisplayName, Email string
	OrgAdmin               bool
}

var (
	FakeAda   = FakeUser{ID: "u-00000000000000a1", DisplayName: "Ada Example", Email: "ada@example.test"}
	FakeGrace = FakeUser{ID: "u-00000000000000a2", DisplayName: "Grace Example", Email: "grace@example.test"}
	FakeLinus = FakeUser{ID: "u-00000000000000a3", DisplayName: "Linus Example", Email: "linus@example.test"}
)

const (
	fakeOrgName  = "Example Org"
	fakeTenantID = "00000000-0000-0000-0000-00000000000e"
	// defaultReservationTTL is contract §7.9's 15 minutes.
	defaultReservationTTL = 15 * time.Minute
)

// fakeWorld is the one state every view shares. Every field is guarded by mu.
type fakeWorld struct {
	mu          sync.Mutex
	now         func() time.Time
	ttl         time.Duration
	quota       int64 // the per-project blob quota in bytes
	signedOut   bool
	unavailable bool
	calls       []string
	seq         int64 // creation order: the tiebreak behind every "oldest first"

	org      hubapi.Org
	users    map[string]hubapi.User // by id; provisioned on first sight
	teams    map[string]*fakeTeam
	invites  map[string]*fakeInvite
	projects map[string]*fakeProject
	briefs   map[string]*fakeBrief
	events   []*fakeEvent // the org's audit log; events[i].seq is i+1
	releases map[string]*fakeRelease

	blobs     map[hubapi.Digest][]byte                  // the org's bytes, stored once
	uses      map[string]map[hubapi.Digest]*fakeBlobUse // by project id: its associations
	approvals map[string]*fakeApproval
	links     map[string]*fakeLink
	evidence  map[string]*fakeEvidence
}

// NewFake is the in-memory Hub (this task and task 05).
func NewFake() *Fake {
	w := &fakeWorld{
		now:      time.Now,
		ttl:      defaultReservationTTL,
		quota:    hubapi.DefaultProjectQuota,
		org:      hubapi.Org{ID: ids.Wide(hubapi.PrefixOrg), DisplayName: fakeOrgName, TenantID: fakeTenantID},
		users:    map[string]hubapi.User{},
		teams:    map[string]*fakeTeam{},
		invites:  map[string]*fakeInvite{},
		projects: map[string]*fakeProject{},
		briefs:   map[string]*fakeBrief{},
		releases: map[string]*fakeRelease{},

		blobs:     map[hubapi.Digest][]byte{},
		uses:      map[string]map[hubapi.Digest]*fakeBlobUse{},
		approvals: map[string]*fakeApproval{},
		links:     map[string]*fakeLink{},
		evidence:  map[string]*fakeEvidence{},
	}
	return &Fake{w: w, as: FakeAda}
}

// As returns a view of the same Hub acting as u. A user is provisioned on first
// sight, as resolve_principal does (contract §5.1 step 5): at the view's first
// call, which also refreshes the display name and email each time they change.
func (f *Fake) As(u FakeUser) *Fake { return &Fake{w: f.w, as: u} }

// SetNow replaces the clock (default time.Now), for reservation expiry. The
// fake reads it under its lock; nil restores time.Now.
func (f *Fake) SetNow(now func() time.Time) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	if now == nil {
		now = time.Now
	}
	f.w.now = now
}

// SetSignedOut makes every call return ErrSignedOut until cleared. It is one
// switch for every view. With SetUnavailable also on, signed out wins: with no
// token, no request is made.
func (f *Fake) SetSignedOut(on bool) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.signedOut = on
}

// SetUnavailable makes every call return an error wrapping ErrUnavailable until cleared.
func (f *Fake) SetUnavailable(on bool) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.unavailable = on
}

// SetReservationTTL overrides the 15-minute default (contract §7.9).
func (f *Fake) SetReservationTTL(d time.Duration) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.ttl = d
}

// SetProjectQuota overrides the 1 GiB per-project blob quota (contract §7.6,
// PANACEA_APP_HUB_PROJECT_QUOTA_BYTES), so a test reaches it with small blobs.
func (f *Fake) SetProjectQuota(bytes int64) {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	f.w.quota = bytes
}

// Calls lists "Method(arg0, …)" for every call made through any view, oldest
// first: refused and switched calls too. The context is not listed; a string
// argument is written as it is, an integer in decimal, anything else as JSON.
func (f *Fake) Calls() []string {
	f.w.mu.Lock()
	defer f.w.mu.Unlock()
	return slices.Clone(f.w.calls)
}

// enter is the first step of every call, with f.w.mu held: it logs the call,
// applies the switches, then resolves the principal as resolve_principal does
// (contract §5.1 step 5) and returns the caller's user id.
func (f *Fake) enter(method string, args ...any) (me string, err error) {
	w := f.w
	w.calls = append(w.calls, method+"("+formatArgs(args)+")")
	switch {
	case w.signedOut:
		return "", ErrSignedOut
	case w.unavailable:
		return "", fmt.Errorf("%w: the fake Hub is switched unavailable", ErrUnavailable)
	}
	_, seen := w.users[f.as.ID]
	w.users[f.as.ID] = hubapi.User{ID: f.as.ID, DisplayName: f.as.DisplayName, Email: strings.ToLower(f.as.Email)}
	if !seen {
		w.emit(f.as.ID, "auth.user_provisioned", "", "", f.as.ID, "tenantId", fakeTenantID)
	}
	w.sweep()
	return f.as.ID, nil
}

func formatArgs(args []any) string {
	parts := make([]string, len(args))
	for i, a := range args {
		switch v := reflect.ValueOf(a); v.Kind() {
		case reflect.String:
			parts[i] = v.String()
		case reflect.Int, reflect.Int64:
			parts[i] = strconv.FormatInt(v.Int(), 10)
		default:
			b, err := json.Marshal(a)
			if err != nil {
				b = []byte(fmt.Sprintf("%v", a))
			}
			parts[i] = string(b)
		}
	}
	return strings.Join(parts, ", ")
}

// stamp is now in the Hub's time format (contract §1).
func (w *fakeWorld) stamp() string { return hubapi.FormatTime(w.now()) }

// next is the next creation number.
func (w *fakeWorld) next() int64 {
	w.seq++
	return w.seq
}

// ── the Hub's refusals (contract §4.2) ──────────────────────────────────────

// problem is an *Error with details from kv ("key", "value", …). It returns
// error, never a typed nil, so no caller can return a nil *Error as non-nil.
func problem(status int, code, message string, kv ...string) error {
	var details map[string]string
	if len(kv) > 0 {
		details = make(map[string]string, len(kv)/2)
		for i := 0; i+1 < len(kv); i += 2 {
			details[kv[i]] = kv[i+1]
		}
	}
	return &Error{Status: status, Problem: httpx.Problem{Error: code, Message: message, Details: details}}
}

func badRequest(field, message string) error {
	return problem(400, hubapi.CodeBadRequest, message, "field", field)
}

// forbidden is 403 for a caller who can see the object and lacks the role.
func forbidden(required string) error {
	return problem(403, hubapi.CodeForbidden, "Your role does not allow that; it needs "+required+".", "required", required)
}

// rank orders the roles viewer < member < admin; anything else is 0.
func rank(r hubapi.Role) int {
	switch r {
	case hubapi.RoleViewer:
		return 1
	case hubapi.RoleMember:
		return 2
	case hubapi.RoleAdmin:
		return 3
	}
	return 0
}

// need is forbidden unless have is at least want.
func need(have, want hubapi.Role) error {
	if rank(have) < rank(want) {
		return forbidden(string(want))
	}
	return nil
}

// ── order and pages (contract §1, "Lists") ──────────────────────────────────

type ordered interface{ order() int64 }

// inOrder is m's rows in creation order.
func inOrder[T ordered](m map[string]T) []T {
	out := make([]T, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	slices.SortFunc(out, func(a, b T) int { return cmp.Compare(a.order(), b.order()) })
	return out
}

// pageLimit is ?limit=: 0 is the default 50; 1–200 as asked; else 400.
func pageLimit(n int) (int, error) {
	switch {
	case n == 0:
		return 50, nil
	case n < 0 || n > 200:
		return 0, badRequest("limit", "A limit is 1 to 200.")
	}
	return n, nil
}

// pageOf renders rows up to limit; next is the last item's cursor when more remain.
func pageOf[T, R any](rows []T, limit int, cursor func(T) string, render func(T) R) hubapi.Page[R] {
	out := hubapi.Page[R]{Items: []R{}}
	for i, r := range rows {
		if i == limit {
			c := cursor(rows[i-1])
			out.Next = &c
			break
		}
		out.Items = append(out.Items, render(r))
	}
	return out
}
