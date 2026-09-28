package realmfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// realmPath is the realm keycloak-config-cli applies in-cluster (env task 29).
const realmPath = "../../../deploy/keycloak/realm/panacea-app.json"

func rawRealm(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(realmPath))
	if err != nil {
		t.Fatalf("read %s: %v", realmPath, err)
	}
	return string(b)
}

func realm(t *testing.T) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(rawRealm(t)), &m); err != nil {
		t.Fatalf("parse %s: %v", realmPath, err)
	}
	return m
}

// list returns m[key] as a list of JSON objects. Nothing is found by position.
func list(t *testing.T, m map[string]any, key string) []map[string]any {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		t.Fatalf("%q is missing", key)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is not a list, it is %T", key, raw)
	}
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		o, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is not an object, it is %T", key, i, it)
		}
		out = append(out, o)
	}
	return out
}

// find returns the one entry of items whose field equals want.
func find(t *testing.T, items []map[string]any, field, want, where string) map[string]any {
	t.Helper()
	var hit map[string]any
	n := 0
	for _, it := range items {
		if s, _ := it[field].(string); s == want {
			hit, n = it, n+1
		}
	}
	if n != 1 {
		t.Fatalf("%s: want exactly one entry with %s=%q, found %d", where, field, want, n)
	}
	return hit
}

func obj(t *testing.T, m map[string]any, key, where string) map[string]any {
	t.Helper()
	o, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("%s: %q is not an object, it is %T", where, key, m[key])
	}
	return o
}

func str(t *testing.T, m map[string]any, key, where string) string {
	t.Helper()
	s, ok := m[key].(string)
	if !ok {
		t.Fatalf("%s: %q is not a string, it is %T", where, key, m[key])
	}
	return s
}

func wantStr(t *testing.T, m map[string]any, key, want, where string) {
	t.Helper()
	if got := str(t, m, key, where); got != want {
		t.Errorf("%s: %s = %q, want %q", where, key, got, want)
	}
}

func wantBool(t *testing.T, m map[string]any, key string, want bool, where string) {
	t.Helper()
	got, ok := m[key].(bool)
	if !ok {
		t.Fatalf("%s: %q is not a bool, it is %T", where, key, m[key])
	}
	if got != want {
		t.Errorf("%s: %s = %v, want %v", where, key, got, want)
	}
}

func wantNum(t *testing.T, m map[string]any, key string, want float64, where string) {
	t.Helper()
	got, ok := m[key].(float64)
	if !ok {
		t.Fatalf("%s: %q is not a number, it is %T", where, key, m[key])
	}
	if got != want {
		t.Errorf("%s: %s = %v, want %v", where, key, got, want)
	}
}

// strings returns m[key] as a sorted []string; a missing key is the empty list.
func strSlice(t *testing.T, m map[string]any, key, where string) []string {
	t.Helper()
	raw, ok := m[key]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s: %q is not a list, it is %T", where, key, raw)
	}
	out := make([]string, 0, len(items))
	for i, it := range items {
		s, ok := it.(string)
		if !ok {
			t.Fatalf("%s: %s[%d] is not a string, it is %T", where, key, i, it)
		}
		out = append(out, s)
	}
	return out
}

func wantSet(t *testing.T, got, want []string, where string) {
	t.Helper()
	g, w := append([]string(nil), got...), append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	if strings.Join(g, ",") != strings.Join(w, ",") {
		t.Errorf("%s: got %v, want %v", where, g, w)
	}
}

func client(t *testing.T, r map[string]any, id string) map[string]any {
	t.Helper()
	return find(t, list(t, r, "clients"), "clientId", id, "clients")
}

func flow(t *testing.T, r map[string]any, alias string) map[string]any {
	t.Helper()
	return find(t, list(t, r, "authenticationFlows"), "alias", alias, "authenticationFlows")
}

var envSubst = regexp.MustCompile(`^\$\(env:[A-Z_]+\)$`)

var guid = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// A file the Job refuses at deploy, or a realm that serves its own sign-up page.
func TestTheRealmFileParses(t *testing.T) {
	r := realm(t)
	wantStr(t, r, "realm", "panacea-app", "realm")
	wantStr(t, r, "sslRequired", "all", "realm")
	wantBool(t, r, "enabled", true, "realm")
	wantBool(t, r, "registrationAllowed", false, "realm")
	wantBool(t, r, "resetPasswordAllowed", false, "realm")
	wantBool(t, r, "loginWithEmailAllowed", false, "realm")
}

// A secret on a laptop, a localhost or wildcard redirect, a password grant,
// or 30 minutes of revocation latency (ruling section 6).
func TestDesktopIsAPublicPKCEClientOnLoopback(t *testing.T) {
	c := client(t, realm(t), "panacea-app-desktop")
	const where = "client panacea-app-desktop"
	wantBool(t, c, "publicClient", true, where)
	if _, ok := c["secret"]; ok {
		t.Errorf("%s: carries a secret; a public client must not", where)
	}
	wantBool(t, c, "standardFlowEnabled", true, where)
	wantBool(t, c, "implicitFlowEnabled", false, where)
	wantBool(t, c, "directAccessGrantsEnabled", false, where)
	wantBool(t, c, "serviceAccountsEnabled", false, where)
	wantBool(t, c, "consentRequired", false, where)
	wantSet(t, strSlice(t, c, "redirectUris", where), []string{"http://127.0.0.1/callback"}, where+" redirectUris")
	wantSet(t, strSlice(t, c, "webOrigins", where), nil, where+" webOrigins")
	a := obj(t, c, "attributes", where)
	wantStr(t, a, "pkce.code.challenge.method", "S256", where+" attributes")
	wantStr(t, a, "oauth2.device.authorization.grant.enabled", "false", where+" attributes")
	wantStr(t, a, "access.token.lifespan", "600", where+" attributes")
}

// A realm password form or SSO cookie that mints a token without Entra, or a
// second IdP that a client's kc_idp_hint could pick.
func TestEveryBrowserSignInGoesToEntra(t *testing.T) {
	r := realm(t)
	c := client(t, r, "panacea-app-desktop")
	over := obj(t, c, "authenticationFlowBindingOverrides", "client panacea-app-desktop")
	wantStr(t, over, "browser", "panacea-app desktop browser", "desktop flow overrides")

	f := flow(t, r, "panacea-app desktop browser")
	const where = "flow panacea-app desktop browser"
	wantBool(t, f, "topLevel", true, where)
	execs := list(t, f, "authenticationExecutions")
	if len(execs) != 1 {
		t.Fatalf("%s: has %d executions, want exactly 1", where, len(execs))
	}
	e := execs[0]
	wantStr(t, e, "authenticator", "identity-provider-redirector", where)
	wantStr(t, e, "requirement", "REQUIRED", where)
	cfgName := str(t, e, "authenticatorConfig", where)
	cfg := find(t, list(t, r, "authenticatorConfig"), "alias", cfgName, "authenticatorConfig")
	wantStr(t, obj(t, cfg, "config", "authenticatorConfig "+cfgName), "defaultProvider", "entra", "authenticatorConfig "+cfgName)

	for _, banned := range []string{"auth-cookie", "auth-username-password-form", "auth-otp-form", "conditional-user-configured"} {
		if a, _ := e["authenticator"].(string); a == banned {
			t.Errorf("%s: holds %q, so a browser sign-in can skip Entra", where, banned)
		}
	}

	idps := list(t, r, "identityProviders")
	if len(idps) != 1 {
		t.Fatalf("identityProviders: %d entries, want exactly 1 (kc_idp_hint can name any of them)", len(idps))
	}
	wantStr(t, idps[0], "alias", "entra", "identityProviders[0]")
	wantBool(t, idps[0], "enabled", true, "identityProviders[0]")
}

var audienceScopes = []string{"panacea-app-hub", "panacea-app-broker", "panacea-app-gateway", "panacea-app-fleet"}

// Groups in every token, or no offline_access, so the silent refresh dies overnight.
func TestDesktopScopesAreExactlyTheRulings(t *testing.T) {
	r := realm(t)
	c := client(t, r, "panacea-app-desktop")
	const where = "client panacea-app-desktop"
	wantSet(t, strSlice(t, c, "optionalClientScopes", where), append([]string{"offline_access"}, audienceScopes...), where+" optionalClientScopes")
	wantSet(t, strSlice(t, c, "defaultClientScopes", where),
		[]string{"basic", "profile", "email", "roles", "web-origins", "acr"}, where+" defaultClientScopes")
	if strings.Contains(rawRealm(t), "groups") {
		t.Errorf("the realm file names groups; Hub membership is the Hub's own (ADR-H1 section 6.6)")
	}
}

// A broker token the Hub also accepts: the confused deputy.
func TestEachAudienceScopeNamesItself(t *testing.T) {
	r := realm(t)
	scopes := list(t, r, "clientScopes")
	if len(scopes) != len(audienceScopes) {
		t.Fatalf("clientScopes: %d entries, want exactly %d", len(scopes), len(audienceScopes))
	}
	for _, name := range audienceScopes {
		s := find(t, scopes, "name", name, "clientScopes")
		where := "clientScope " + name
		wantStr(t, s, "protocol", "openid-connect", where)
		wantStr(t, obj(t, s, "attributes", where), "include.in.token.scope", "true", where+" attributes")
		ms := list(t, s, "protocolMappers")
		if len(ms) != 1 {
			t.Fatalf("%s: %d protocol mappers, want exactly 1", where, len(ms))
		}
		wantStr(t, ms[0], "protocolMapper", "oidc-audience-mapper", where)
		cfg := obj(t, ms[0], "config", where)
		wantStr(t, cfg, "included.custom.audience", name, where+" audience")
		wantStr(t, cfg, "access.token.claim", "true", where+" audience")
		wantStr(t, cfg, "id.token.claim", "false", where+" audience")
	}
}

// A person keyed on Keycloak's sub, which changes when the realm does.
func TestPersonClaimsCarryTheEntraIds(t *testing.T) {
	c := client(t, realm(t), "panacea-app-desktop")
	ms := list(t, c, "protocolMappers")
	for _, w := range []struct{ mapper, attr, claim string }{
		{"oid", "entra_oid", "oid"},
		{"tid", "entra_tid", "tid"},
	} {
		m := find(t, ms, "name", w.mapper, "desktop protocolMappers")
		where := "desktop mapper " + w.mapper
		wantStr(t, m, "protocolMapper", "oidc-usermodel-attribute-mapper", where)
		cfg := obj(t, m, "config", where)
		wantStr(t, cfg, "user.attribute", w.attr, where)
		wantStr(t, cfg, "claim.name", w.claim, where)
		wantStr(t, cfg, "jsonType.label", "String", where)
		wantStr(t, cfg, "access.token.claim", "true", where)
		wantStr(t, cfg, "id.token.claim", "true", where)
	}
	m := find(t, ms, "name", "roles", "desktop protocolMappers")
	const where = "desktop mapper roles"
	wantStr(t, m, "protocolMapper", "oidc-usermodel-realm-role-mapper", where)
	cfg := obj(t, m, "config", where)
	wantStr(t, cfg, "claim.name", "roles", where)
	wantStr(t, cfg, "multivalued", "true", where)
	wantStr(t, cfg, "access.token.claim", "true", where)
	wantStr(t, cfg, "id.token.claim", "false", where)
}

// A user who never gets entra_oid (IMPORT writes only at creation), or a stale
// or self-edited email surviving a login.
func TestIdPMappersForceTheEntraIds(t *testing.T) {
	ms := list(t, realm(t), "identityProviderMappers")
	for _, w := range []struct{ name, claim, attr string }{
		{"entra_oid", "oid", "entra_oid"},
		{"entra_tid", "tid", "entra_tid"},
		{"email", "email", "email"},
	} {
		m := find(t, ms, "name", w.name, "identityProviderMappers")
		where := "idp mapper " + w.name
		wantStr(t, m, "identityProviderAlias", "entra", where)
		wantStr(t, m, "identityProviderMapper", "oidc-user-attribute-idp-mapper", where)
		cfg := obj(t, m, "config", where)
		wantStr(t, cfg, "claim", w.claim, where)
		wantStr(t, cfg, "user.attribute", w.attr, where)
		wantStr(t, cfg, "syncMode", "FORCE", where)
	}
}

// A person rewriting their own oid or email in the account console, becoming
// someone else or binding a colleague's invite.
func TestOnlyAnAdminCanEditTheEntraIds(t *testing.T) {
	r := realm(t)
	up := obj(t, r, "userProfile", "realm")
	policy := str(t, up, "unmanagedAttributePolicy", "userProfile")
	if policy == "ENABLED" {
		t.Errorf("userProfile: unmanagedAttributePolicy is ENABLED, so an undeclared attribute is user-editable")
	}
	attrs := list(t, up, "attributes")
	for _, w := range []struct {
		name string
		view []string
	}{
		{"entra_oid", []string{"admin"}},
		{"entra_tid", []string{"admin"}},
		{"email", []string{"admin", "user"}},
	} {
		a := find(t, attrs, "name", w.name, "userProfile attributes")
		where := "userProfile " + w.name
		perms := obj(t, a, "permissions", where)
		wantSet(t, strSlice(t, perms, "edit", where), []string{"admin"}, where+" edit")
		wantSet(t, strSlice(t, perms, "view", where), w.view, where+" view")
	}
}

// Any Entra tenant signing in, an Entra token stored in the realm, or a click.
func TestTheIdPIsPinnedToOneTenant(t *testing.T) {
	r := realm(t)
	idp := find(t, list(t, r, "identityProviders"), "alias", "entra", "identityProviders")
	const where = "idp entra"
	wantStr(t, idp, "providerId", "oidc", where)
	wantBool(t, idp, "trustEmail", true, where)
	wantBool(t, idp, "storeToken", false, where)
	wantBool(t, idp, "linkOnly", false, where)
	flow(t, r, str(t, idp, "firstBrokerLoginFlowAlias", where))

	cfg := obj(t, idp, "config", where)
	const tenant = "$(env:PANACEA_APP_ENTRA_TENANT_ID)"
	for _, key := range []string{"authorizationUrl", "tokenUrl", "issuer", "jwksUrl"} {
		if got := str(t, cfg, key, where); !strings.Contains(got, tenant) {
			t.Errorf("%s: %s = %q, which is not pinned to %s", where, key, got, tenant)
		}
	}
	wantStr(t, cfg, "clientAuthMethod", "client_secret_post", where)
	wantStr(t, cfg, "useJwksUrl", "true", where)
	wantStr(t, cfg, "validateSignature", "true", where)
	wantStr(t, cfg, "filteredByClaim", "true", where)
	wantStr(t, cfg, "claimFilterName", "tid", where)
	wantStr(t, cfg, "claimFilterValue", tenant, where)
	wantStr(t, cfg, "loginHint", "true", where)
	wantStr(t, cfg, "syncMode", "IMPORT", where)
	if _, ok := cfg["prompt"]; ok {
		t.Errorf("%s: sets prompt, which puts a click in front of the person (ruling section 3)", where)
	}
}

// A profile-review page at first sign-in.
func TestFirstLoginHasNoReviewStep(t *testing.T) {
	r := realm(t)
	if strings.Contains(rawRealm(t), "idp-review-profile") {
		t.Errorf("the realm file names idp-review-profile, which is a click at first sign-in")
	}
	f := flow(t, r, "panacea-app first broker login")
	const where = "flow panacea-app first broker login"
	execs := list(t, f, "authenticationExecutions")
	if len(execs) != 2 {
		t.Fatalf("%s: %d executions, want exactly 2", where, len(execs))
	}
	first := find(t, execs, "authenticator", "idp-create-user-if-unique", where)
	wantStr(t, first, "requirement", "ALTERNATIVE", where)
	second := find(t, execs, "flowAlias", "panacea-app handle existing account", where)
	wantStr(t, second, "requirement", "ALTERNATIVE", where)
	wantBool(t, second, "authenticatorFlow", true, where)

	sub := flow(t, r, "panacea-app handle existing account")
	const subWhere = "flow panacea-app handle existing account"
	wantBool(t, sub, "topLevel", false, subWhere)
	subExecs := list(t, sub, "authenticationExecutions")
	if len(subExecs) != 1 {
		t.Fatalf("%s: %d executions, want exactly 1", subWhere, len(subExecs))
	}
	wantStr(t, subExecs[0], "authenticator", "idp-auto-link", subWhere)
	wantStr(t, subExecs[0], "requirement", "REQUIRED", subWhere)
}

// A service token carrying oid, or a gateway token the broker could accept.
func TestServiceClientsCarryNoPerson(t *testing.T) {
	r := realm(t)
	for _, w := range []struct {
		id       string
		optional []string
	}{
		{"panacea-app-fleet", []string{"panacea-app-hub", "panacea-app-broker"}},
		{"panacea-app-gateway", []string{"panacea-app-hub"}},
	} {
		c := client(t, r, w.id)
		where := "client " + w.id
		wantBool(t, c, "publicClient", false, where)
		wantStr(t, c, "clientAuthenticatorType", "client-secret", where)
		wantBool(t, c, "serviceAccountsEnabled", true, where)
		wantBool(t, c, "standardFlowEnabled", false, where)
		wantBool(t, c, "implicitFlowEnabled", false, where)
		wantBool(t, c, "directAccessGrantsEnabled", false, where)
		wantSet(t, strSlice(t, c, "redirectUris", where), nil, where+" redirectUris")
		wantSet(t, strSlice(t, c, "webOrigins", where), nil, where+" webOrigins")
		wantSet(t, strSlice(t, c, "defaultClientScopes", where), nil, where+" defaultClientScopes")
		wantSet(t, strSlice(t, c, "optionalClientScopes", where), w.optional, where+" optionalClientScopes")
		if ms, ok := c["protocolMappers"]; ok {
			if l, _ := ms.([]any); len(l) != 0 {
				t.Errorf("%s: has %d protocol mappers; a service token carries no person", where, len(l))
			}
		}
	}
}

// walk calls fn for every (key, value) pair in the decoded tree.
func walk(v any, fn func(key string, val any)) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			fn(k, val)
			walk(val, fn)
		}
	case []any:
		for _, val := range t {
			walk(val, fn)
		}
	}
}

// A secret or a tenant id committed to the repository.
func TestNothingSecretIsTyped(t *testing.T) {
	n := 0
	walk(realm(t), func(key string, val any) {
		if key != "secret" && key != "clientSecret" {
			return
		}
		n++
		s, ok := val.(string)
		if !ok {
			t.Fatalf("%q is not a string, it is %T", key, val)
		}
		if !envSubst.MatchString(s) {
			t.Errorf("%q = %q, which is not an $(env:NAME) substitution", key, s)
		}
	})
	if n != 3 {
		t.Errorf("found %d secret keys, want 3 (Fleet, gateway, the Entra IdP)", n)
	}
	if got := guid.FindString(rawRealm(t)); got != "" {
		t.Errorf("the realm file holds the GUID %q; ids come in through $(env:NAME)", got)
	}
}

// A person disabled in Entra keeping cloud access for 60 days (ruling section 6).
func TestLeaversLapseWithinAWeek(t *testing.T) {
	r := realm(t)
	wantBool(t, r, "offlineSessionMaxLifespanEnabled", true, "realm")
	wantNum(t, r, "offlineSessionMaxLifespan", 5184000, "realm")
	wantNum(t, r, "ssoSessionMaxLifespan", 86400, "realm")
	c := client(t, r, "panacea-app-desktop")
	wantStr(t, obj(t, c, "attributes", "client panacea-app-desktop"),
		"client.offline.session.max.lifespan", "604800", "client panacea-app-desktop attributes")
}

// No org admin able to erase (hub ADR-H1 G6).
func TestOrgAdminIsARealmRole(t *testing.T) {
	roles := obj(t, realm(t), "roles", "realm")
	find(t, list(t, roles, "realm"), "name", "hub.org_admin", "realm roles")
}
