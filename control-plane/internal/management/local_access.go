package management

import (
	"bytes"
	"crypto/subtle"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const localAccessHeader = "X-Luci-Local-Access-Token"

func validLocalAccessToken(token string) bool {
	if len(token) < 32 || len(token) > 256 {
		return false
	}
	for _, b := range []byte(token) {
		if b < 33 || b > 126 {
			return false
		}
	}
	return true
}

// A loopback address is not authority. The owning launcher must retain this
// per-process capability; it is never returned through a public browser API.
func (g *Gateway) requireLocalAccess(w http.ResponseWriter, r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	provided := r.Header.Get(localAccessHeader)
	r.Header.Del(localAccessHeader)
	if err != nil || ip == nil || !ip.IsLoopback() || !validLocalAccessToken(g.opts.LocalAccessToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(g.opts.LocalAccessToken)) != 1 {
		fail(w, 403, "local_access_required", "Reopen Luci using its local launcher on the computer that owns this workspace.")
		return false
	}
	return true
}

func (g *Gateway) removePortableOwnerAccess() {
	if !g.opts.PortableOwnerAccess {
		return
	}
	path := filepath.Join(g.dir, "owner-access-token")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	data, err := os.ReadFile(path)
	if err == nil && bytes.Equal(data, []byte(g.opts.LocalAccessToken+"\n")) {
		_ = os.Remove(path)
	}
}

func (g *Gateway) localOwner(w http.ResponseWriter, r *http.Request) {
	if !g.requireLocalAccess(w, r) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	var owner *User
	if row, ok := g.state.Users[g.state.OwnerID]; ok {
		copy := row.User
		owner = &copy
	}
	jsonReply(w, 200, map[string]any{"owner": owner})
}

// Copy persistent mutable collections before a transaction. Failed disk writes
// must not change live credentials, consume an invitation or strand the owner.
func cloneIdentityState(source storedState) storedState {
	out := source
	out.Users = make(map[string]userRecord, len(source.Users))
	for k, v := range source.Users {
		out.Users[k] = v
	}
	out.Sessions = make(map[string]sessionRecord, len(source.Sessions))
	for k, v := range source.Sessions {
		out.Sessions[k] = v
	}
	out.Events = append([]AuditEvent{}, source.Events...)
	out.Invitations = make(map[string]invitationRecord, len(source.Invitations))
	for k, v := range source.Invitations {
		out.Invitations[k] = v
	}
	return out
}

func (g *Gateway) localAccount(w http.ResponseWriter, r *http.Request) {
	if !g.requireLocalAccess(w, r) {
		return
	}
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !validIdentity(req.Username, req.DisplayName, req.Password) {
		fail(w, 400, "invalid_identity", "Use a valid username, display name, and password of 12–1,024 characters.")
		return
	}
	if !g.allowLogin(r, "local-account") {
		fail(w, 429, "rate_limited", "Try again later.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	hash := passwordHash(req.Password)
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.state.Users) == 0 {
		fail(w, 409, "bootstrap_required", "Create the first workspace owner before adding another account.")
		return
	}
	if len(g.state.Users) >= 1000 {
		fail(w, 409, "user_limit", "This instance supports at most 1,000 users.")
		return
	}
	for _, row := range g.state.Users {
		if strings.EqualFold(row.User.Username, req.Username) {
			fail(w, 409, "username_exists", "Choose another username.")
			return
		}
	}
	before := g.state
	g.state = cloneIdentityState(before)
	u := User{ID: opaqueID("user-"), Username: strings.ToLower(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), Role: "super-admin"}
	g.state.Users[u.ID] = userRecord{User: u, PasswordHash: hash}
	g.auditLocked(u.ID, "user.create.local", u.ID, "succeeded")
	token, err := g.issueSessionLocked(u)
	if err != nil {
		g.state = before
		fail(w, 500, "save_failed", "The account could not be saved.")
		return
	}
	jsonReply(w, 201, map[string]any{"token": token, "user": u})
}

func (g *Gateway) recoverLocalOwner(w http.ResponseWriter, r *http.Request) {
	if !g.requireLocalAccess(w, r) {
		return
	}
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.NewPassword) < 12 || len(req.NewPassword) > 1024 {
		fail(w, 400, "invalid_password", "Use a password of 12–1,024 characters.")
		return
	}
	if !g.allowLogin(r, "local-recovery") {
		fail(w, 429, "rate_limited", "Try again later.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	hash := passwordHash(req.NewPassword)
	g.mu.Lock()
	row, ok := g.state.Users[g.state.OwnerID]
	if !ok {
		g.mu.Unlock()
		fail(w, 409, "bootstrap_required", "Create the first workspace owner.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row.PasswordHash = hash
	// OS-profile ownership can restore only the original installation owner.
	row.User.Disabled = false
	row.User.Role = "super-admin"
	g.state.Users[row.User.ID] = row
	for key, s := range g.state.Sessions {
		if s.UserID == row.User.ID {
			delete(g.state.Sessions, key)
		}
	}
	g.auditLocked(row.User.ID, "auth.owner.recover.local", row.User.ID, "succeeded")
	token, err := g.issueSessionLocked(row.User)
	if err != nil {
		g.state = before
		g.mu.Unlock()
		fail(w, 500, "save_failed", "Owner access could not be saved.")
		return
	}
	g.closeSocketsLocked(row.User.ID, "")
	for key, t := range g.tickets {
		if t.UserID == row.User.ID {
			delete(g.tickets, key)
		}
	}
	rt := g.runtimes[row.User.ID]
	g.mu.Unlock()
	if rt != nil {
		rt.stop()
	}
	jsonReply(w, 200, map[string]any{"token": token, "user": row.User})
}
