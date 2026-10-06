package management

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Object names are data below a fixed route, unlike the team/project/bucket
// selectors that establish authority. Encoded object segments must survive the
// management proxy without allowing an encoded selector to change that scope.
var storageObjectPrefix = regexp.MustCompile(`^/v1/(?:teams/team-[A-Za-z0-9_-]{22}/workspace/v1/)?platform/projects/[A-Za-z0-9_-]{1,128}/data/buckets/[a-z0-9][a-z0-9._-]{0,62}/objects/`)

func canonicalStorageObjectPath(r *http.Request) bool {
	if r.URL == nil || (r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodPost) {
		return false
	}
	escaped := r.URL.EscapedPath()
	if r.URL.RawPath != "" && escaped != r.URL.RawPath {
		return false
	}
	prefix := storageObjectPrefix.FindString(escaped)
	if prefix == "" || !strings.HasPrefix(r.URL.Path, prefix) {
		return false
	}
	name, err := url.PathUnescape(strings.TrimPrefix(escaped, prefix))
	if err != nil || prefix+name != r.URL.Path || name == "" || len(name) > 512 || strings.ContainsAny(name, "\\\x00\r\n") {
		return false
	}
	for _, segment := range strings.Split(name, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}
