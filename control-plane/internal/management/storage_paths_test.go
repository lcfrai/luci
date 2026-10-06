package management

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStorageObjectEscapingNeverChangesAuthoritySelectors(t *testing.T) {
	base := "/v1/platform/projects/p-alpha/data/buckets/assets/objects/"
	for _, name := range []string{"nested%2Ffile%20name.txt", "%E2%9C%93%20report.txt", "version%252Fopaque.txt", "%66ile.txt", "nested%2ffile.txt"} {
		r := httptest.NewRequest("POST", base+name, nil)
		scope, ok := ClassifyTeamRoute(r)
		if !canonicalStorageObjectPath(r) || !ok || scope.ProjectID != "p-alpha" {
			t.Fatalf("safe object path lost its project: %q %+v %v", name, scope, ok)
		}
	}
	for _, endpoint := range []string{
		base + "%2e%2e/private", base + "nested%2F%2e%2e/private", base + "nested%2F%2Fprivate", base + "bad%5Cname", base + "bad%00name",
		strings.Replace(base, "p-alpha", "%70-alpha", 1) + "file", strings.Replace(base, "assets", "%61ssets", 1) + "file",
		strings.Replace(base, "p-alpha", "p-alpha%2Fother", 1) + "file", "/v1/platform/projects/p-alpha/data/functions/name%2Faction/source",
	} {
		r := httptest.NewRequest("POST", endpoint, nil)
		if canonicalStorageObjectPath(r) {
			t.Fatalf("escaped authority/traversal was accepted: %q", endpoint)
		}
		if _, known := ClassifyTeamRoute(r); known {
			t.Fatalf("escaped authority/traversal acquired a team route: %q", endpoint)
		}
	}
}

func TestTeamStorageObjectProxyPreservesOnlyAuthorizedObjectEscapes(t *testing.T) {
	f := newTeamProxyFixture(t)
	inner := "/v1/platform/projects/p-alpha/data/buckets/assets/objects/nested%2Ffile%20name.txt"
	var escaped string
	f.g.opts.TeamHandler = func(context.Context, string) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			escaped = r.URL.EscapedPath()
			w.WriteHeader(204)
		}), nil
	}
	request(t, f.h, "POST", f.path(inner), f.memberToken, nil, 204)
	if escaped != inner {
		t.Fatalf("object path changed across namespace: got %q want %q", escaped, inner)
	}
	escaped = ""
	request(t, f.h, "POST", f.path(strings.Replace(inner, "p-alpha", "p-secret", 1)), f.memberToken, nil, 403)
	if escaped != "" {
		t.Fatal("ungranted project reached object storage")
	}
	request(t, f.h, "POST", f.path(strings.Replace(inner, "p-alpha", "%70-alpha", 1)), f.ownerToken, nil, 400)
	request(t, f.h, "POST", strings.Replace(f.path(inner), f.team.ID, "team-%61"+strings.TrimPrefix(f.team.ID, "team-")[1:], 1), f.ownerToken, nil, 400)
	if escaped != "" {
		t.Fatal("escaped authority selector reached object storage")
	}
}
