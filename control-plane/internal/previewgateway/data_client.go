package previewgateway

import (
	"net/http"
	"strings"
)

// AllowDataClient is shared by local Docker and authenticated cloud bridges.
// Only the Supabase application surface and client credentials may cross it;
// the upstream services still verify JWT signatures and row-level security.
func AllowDataClient(w http.ResponseWriter, r *http.Request, publishableKey string) bool {
	if !dataPath(r.URL.Path) {
		http.NotFound(w, r)
		return false
	}
	if auth := r.Header.Get("Authorization"); auth != "" && (!strings.HasPrefix(auth, "Bearer ") || !jwtClient(strings.TrimPrefix(auth, "Bearer "))) {
		http.Error(w, "Use a Supabase client user token", 401)
		return false
	}
	if key := r.Header.Get("apikey"); key != "" && key != publishableKey {
		http.Error(w, "Use this project's publishable key", 401)
		return false
	}
	q := r.URL.Query()
	if key := q.Get("apikey"); key != "" && key != publishableKey {
		http.Error(w, "Use this project's publishable key", 401)
		return false
	}
	if token := q.Get("access_token"); token != "" && !jwtClient(token) {
		http.Error(w, "Use a Supabase client user token", 401)
		return false
	}
	return true
}
