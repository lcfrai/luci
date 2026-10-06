package management

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

// RFC 9106's memory-constrained Argon2id profile. Two concurrent password
// derivations bound peak memory; a random 128-bit salt is stored per password.
func passwordHash(password string) string {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		panic("operating system randomness unavailable")
	}
	key := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return "argon2id:v19:m65536:t3:p4:" + base64.RawStdEncoding.EncodeToString(salt) + ":" + base64.RawStdEncoding.EncodeToString(key)
}
func passwordMatches(password, encoded string) bool {
	parts := strings.Split(encoded, ":")
	salt := make([]byte, 16)
	want := make([]byte, 32)
	valid := false
	if len(parts) == 7 && strings.Join(parts[:5], ":") == "argon2id:v19:m65536:t3:p4" {
		s, e := base64.RawStdEncoding.DecodeString(parts[5])
		k, e2 := base64.RawStdEncoding.DecodeString(parts[6])
		if e == nil && e2 == nil && len(s) == 16 && len(k) == 32 {
			salt, want, valid = s, k, true
		}
	}
	got := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return subtle.ConstantTimeCompare(got, want) == 1 && valid
}
func (g *Gateway) passwordSlot(w http.ResponseWriter) bool {
	select {
	case g.passwordSlots <- struct{}{}:
		return true
	default:
		fail(w, 429, "authentication_busy", "Authentication is busy. Try again shortly.")
		return false
	}
}
func (g *Gateway) releasePasswordSlot() { <-g.passwordSlots }
func (g *Gateway) allowLogin(r *http.Request, username string) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	keys := []string{"ip:" + hashToken(ip), "user:" + hashToken(strings.ToLower(username))}
	now := time.Now()
	g.mu.Lock()
	defer g.mu.Unlock()
	for key, value := range g.attempts {
		if now.After(value.Until) {
			delete(g.attempts, key)
		}
	}
	if len(g.attempts) > 4096 {
		return false
	}
	for i, key := range keys {
		value := g.attempts[key]
		limit := 12
		if i == 0 {
			limit = 60
		}
		if value.Count >= limit && now.Before(value.Until) {
			return false
		}
	}
	for _, key := range keys {
		value := g.attempts[key]
		if value.Until.IsZero() {
			value.Until = now.Add(10 * time.Minute)
		}
		value.Count++
		g.attempts[key] = value
	}
	return true
}

// The caller stages a cloned identity and restores it if this commit fails.
func (g *Gateway) issueSessionLocked(u User) (string, error) {
	now := time.Now()
	revoked := map[string]string{}
	for key, s := range g.state.Sessions {
		if now.After(s.ExpiresAt) {
			revoked[key] = s.UserID
			delete(g.state.Sessions, key)
		}
	}
	// Keep at most sixteen sessions per account; the oldest expiry is oldest.
	for {
		count, oldest, at := 0, "", now.Add(24*time.Hour)
		for key, s := range g.state.Sessions {
			if s.UserID == u.ID {
				count++
				if s.ExpiresAt.Before(at) {
					oldest, at = key, s.ExpiresAt
				}
			}
		}
		if count < 16 {
			break
		}
		revoked[oldest] = u.ID
		delete(g.state.Sessions, oldest)
	}
	if len(g.state.Sessions) >= 4096 {
		return "", fmt.Errorf("too many active sessions")
	}
	token := randomToken()
	g.state.Sessions[hashToken(token)] = sessionRecord{UserID: u.ID, ExpiresAt: now.Add(12 * time.Hour)}
	if err := g.persistLocked(); err != nil {
		delete(g.state.Sessions, hashToken(token))
		return "", err
	}
	// Eviction is revocation too. Close only connections for the committed
	// removals; a failed sign-in must leave existing terminal sessions usable.
	for sessionHash, userID := range revoked {
		g.closeSocketsLocked(userID, sessionHash)
	}
	for key, ticket := range g.tickets {
		if _, removed := revoked[ticket.SessionHash]; removed {
			delete(g.tickets, key)
		}
	}
	return token, nil
}

// Stage persistent revocation before saving; live connections and workers must
// remain usable if the identity transaction cannot be committed.
func (g *Gateway) revokeSessionsLocked(userID string) {
	for key, s := range g.state.Sessions {
		if s.UserID == userID {
			delete(g.state.Sessions, key)
		}
	}
}
func (g *Gateway) revokeConnectionsLocked(userID string) {
	g.closeSocketsLocked(userID, "")
	for key, t := range g.tickets {
		if t.UserID == userID {
			delete(g.tickets, key)
		}
	}
}
func (g *Gateway) authenticate(r *http.Request) (User, Policy, string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	var id, sessionHash string
	if token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); token != "" && token != r.Header.Get("Authorization") && len(token) <= 256 {
		if s, ok := g.state.Sessions[hashToken(token)]; ok && now.Before(s.ExpiresAt) {
			id = s.UserID
			sessionHash = hashToken(token)
		}
	}
	if id == "" && r.Method == "GET" && validAttachPath(r.URL.Path) && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		value := r.URL.Query().Get("ticket")
		key := hashToken(value)
		t, ok := g.tickets[key]
		if ok {
			delete(g.tickets, key)
		}
		session, sessionExists := g.state.Sessions[t.SessionHash]
		teamCurrent := t.TeamID == "" || g.teamAccessCurrentLocked(t.TeamID, t.UserID, t.ProjectID, t.MemberRevision, t.ProjectRevision)
		if ok && value != "" && t.Path == r.URL.Path && now.Before(t.ExpiresAt) && sessionExists && session.UserID == t.UserID && now.Before(session.ExpiresAt) && teamCurrent {
			id = t.UserID
			sessionHash = t.SessionHash
		}
		query := r.URL.Query()
		query.Del("ticket")
		r.URL.RawQuery = query.Encode()
	}
	record, ok := g.state.Users[id]
	return record.User, g.state.Policy, sessionHash, ok && !record.User.Disabled && !g.closed
}
func (g *Gateway) bootstrap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BootstrapToken string `json:"bootstrapToken"`
		Username       string `json:"username"`
		DisplayName    string `json:"displayName"`
		Password       string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !g.allowLogin(r, "bootstrap") {
		fail(w, 429, "rate_limited", "Too many authentication attempts. Try again later.")
		return
	}
	if !validIdentity(req.Username, req.DisplayName, req.Password) {
		fail(w, 400, "invalid_identity", "Use a 3–64 character username, a display name, and a password of 12–1,024 characters.")
		return
	}
	g.mu.Lock()
	allowed := len(g.state.Users) == 0 && g.bootstrapToken != "" && subtle.ConstantTimeCompare([]byte(req.BootstrapToken), []byte(g.bootstrapToken)) == 1
	g.mu.Unlock()
	if !allowed {
		fail(w, 403, "bootstrap_denied", "The bootstrap token is invalid or this instance is already configured.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	hash := passwordHash(req.Password)
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.state.Users) != 0 {
		fail(w, 409, "already_configured", "This instance already has an owner.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	u := User{ID: opaqueID("user-"), Username: strings.ToLower(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), Role: "super-admin"}
	g.state.OwnerID = u.ID
	g.state.Users[u.ID] = userRecord{User: u, PasswordHash: hash}
	g.auditLocked(u.ID, "instance.bootstrap", u.ID, "succeeded")
	token, err := g.issueSessionLocked(u)
	if err != nil {
		g.state = before
		fail(w, 500, "save_failed", "The identity could not be saved.")
		return
	}
	g.bootstrapToken = ""
	_ = os.Remove(filepath.Join(g.dir, "bootstrap-token"))
	jsonReply(w, 201, map[string]any{"token": token, "user": u})
}
func (g *Gateway) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Username) > 64 || len(req.Password) > 1024 {
		fail(w, 401, "invalid_credentials", "The username or password is incorrect.")
		return
	}
	if !g.allowLogin(r, req.Username) {
		fail(w, 429, "rate_limited", "Too many authentication attempts. Try again later.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	g.mu.Lock()
	var record userRecord
	for _, u := range g.state.Users {
		if strings.EqualFold(u.User.Username, req.Username) {
			record = u
			break
		}
	}
	g.mu.Unlock()
	valid := passwordMatches(req.Password, record.PasswordHash)
	g.mu.Lock()
	defer g.mu.Unlock()
	current, exists := g.state.Users[record.User.ID]
	before := g.state
	g.state = cloneIdentityState(before)
	if !valid || !exists || current.User.Disabled || current.PasswordHash != record.PasswordHash {
		g.auditLocked("", "auth.login", "", "denied")
		if g.persistLocked() != nil {
			g.state = before
		}
		fail(w, 401, "invalid_credentials", "The username or password is incorrect.")
		return
	}
	g.auditLocked(current.User.ID, "auth.login", current.User.ID, "succeeded")
	token, err := g.issueSessionLocked(current.User)
	if err != nil {
		g.state = before
		fail(w, 500, "save_failed", "Sign-in could not be saved.")
		return
	}
	delete(g.attempts, "user:"+hashToken(strings.ToLower(req.Username)))
	jsonReply(w, 200, map[string]any{"token": token, "user": current.User})
}
func (g *Gateway) me(w http.ResponseWriter, r *http.Request) {
	u, p, _ := ActorFromContext(r.Context())
	jsonReply(w, 200, map[string]any{"user": u, "permissions": PermissionsFor(u, p), "policy": p})
}
func (g *Gateway) logout(w http.ResponseWriter, r *http.Request) {
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	before := g.state
	g.state = cloneIdentityState(before)
	sessionHash := hashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	delete(g.state.Sessions, sessionHash)
	g.auditLocked(u.ID, "auth.logout", u.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "Sign-out could not be saved.")
		return
	}
	g.closeSocketsLocked(u.ID, sessionHash)
	for key, t := range g.tickets {
		if t.UserID == u.ID {
			delete(g.tickets, key)
		}
	}
	jsonReply(w, 200, map[string]bool{"signedOut": true})
}
func (g *Gateway) changePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, _, _ := ActorFromContext(r.Context())
	if len(req.CurrentPassword) > 1024 || len(req.NewPassword) < 12 || len(req.NewPassword) > 1024 {
		fail(w, 400, "invalid_password", "Use a new password of 12–1,024 characters.")
		return
	}
	if !g.allowLogin(r, u.Username) {
		fail(w, 429, "rate_limited", "Try again shortly.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	g.mu.Lock()
	record := g.state.Users[u.ID]
	g.mu.Unlock()
	if !passwordMatches(req.CurrentPassword, record.PasswordHash) {
		fail(w, 403, "invalid_credentials", "The current password is incorrect.")
		return
	}
	hash := passwordHash(req.NewPassword)
	g.mu.Lock()
	current := g.state.Users[u.ID]
	if current.PasswordHash != record.PasswordHash || current.User.Disabled {
		g.mu.Unlock()
		fail(w, 409, "identity_changed", "Sign in again before changing your password.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	current.PasswordHash = hash
	g.state.Users[u.ID] = current
	g.revokeSessionsLocked(u.ID)
	g.auditLocked(u.ID, "auth.password", u.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		g.mu.Unlock()
		fail(w, 500, "save_failed", "Password update could not be saved.")
		return
	}
	g.revokeConnectionsLocked(u.ID)
	rt := g.runtimes[u.ID]
	g.mu.Unlock()
	if rt != nil {
		rt.stop()
	}
	jsonReply(w, 200, map[string]bool{"signInRequired": true})
}

var attachPathPattern = regexp.MustCompile(`^/v1/sessions/[A-Za-z0-9_-]+/attach$`)
var teamAttachPathPattern = regexp.MustCompile(`^/v1/teams/(team-[A-Za-z0-9_-]{22})/workspace/v1/sessions/([A-Za-z0-9_-]+)/attach$`)

func validAttachPath(path string) bool {
	return attachPathPattern.MatchString(path) || teamAttachPathPattern.MatchString(path)
}

func (g *Gateway) createSocketTicket(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !validAttachPath(req.Path) {
		fail(w, 400, "invalid_path", "Choose the session's attach path.")
		return
	}
	u, policy, _ := ActorFromContext(r.Context())
	var teamAccess TeamAccess
	if g.managedTeamID != "" {
		target := teamAttachPathPattern.FindStringSubmatch(req.Path)
		if target == nil || target[1] != g.managedTeamID {
			fail(w, 403, "managed_team_required", "Only this installation's assigned team can open a terminal.")
			return
		}
	}
	if target := teamAttachPathPattern.FindStringSubmatch(req.Path); target != nil {
		var err error
		teamAccess, err = g.resolveTeamSocketAccess(r.Context(), u, policy, target[1], target[2])
		if err != nil {
			fail(w, 403, "team_socket_denied", "You need current execution access to this team's project before opening its terminal.")
			return
		}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	for key, t := range g.tickets {
		if now.After(t.ExpiresAt) {
			delete(g.tickets, key)
		}
	}
	if len(g.tickets) >= 4096 {
		fail(w, 429, "ticket_limit", "Too many pending socket connections.")
		return
	}
	ticket := randomToken()
	expires := now.Add(30 * time.Second)
	sessionHash := hashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	session, ok := g.state.Sessions[sessionHash]
	if !ok || session.UserID != u.ID || !now.Before(session.ExpiresAt) {
		fail(w, 401, "sign_in_required", "Sign in again before connecting.")
		return
	}
	if teamAccess.Team.ID != "" && (!g.teamAccessCurrentLocked(teamAccess.Team.ID, u.ID, teamAccess.ProjectID, teamAccess.Member.Revision, teamAccess.ProjectRevision) || !g.teamServiceAllowsExecutionLocked(teamAccess.Team.ID)) {
		fail(w, 403, "team_access_changed", "Team access changed before this terminal ticket could be issued.")
		return
	}
	if session.ExpiresAt.Before(expires) {
		expires = session.ExpiresAt
	}
	g.tickets[hashToken(ticket)] = socketTicket{UserID: u.ID, Path: req.Path, SessionHash: sessionHash, ExpiresAt: expires, TeamID: teamAccess.Team.ID, ProjectID: teamAccess.ProjectID, MemberRevision: teamAccess.Member.Revision, ProjectRevision: teamAccess.ProjectRevision}
	jsonReply(w, 200, map[string]any{"ticket": ticket, "expiresAt": expires.Format(time.RFC3339Nano)})
}
