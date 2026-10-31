package product

import (
	"sort"
	"strings"
)

// An optional allow-list names operator-configured hosts, never provider URLs
// or credentials. Omission preserves the original default single-host route.
func validCloudHostSelection(ids []string, digest, location string) bool {
	if len(ids) == 0 {
		return digest == ""
	}
	if !hexDigest(digest) || location != "cloud-microvm" || len(ids) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if len(id) < 1 || len(id) > 64 || seen[id] || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") {
			return false
		}
		for _, r := range id {
			if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
		seen[id] = true
	}
	return true
}
func normalizedCloudHosts(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := append([]string{}, ids...)
	sort.Strings(out)
	return out
}
func sameCloudHosts(a, b []string) bool {
	left, right := normalizedCloudHosts(a), normalizedCloudHosts(b)
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
