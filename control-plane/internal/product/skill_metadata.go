package product

import (
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Metadata records the editor's attribution and compatibility declarations.
// It neither verifies a licence nor installs or grants a declared tool.
type ProjectSkillMetadata struct {
	License        string   `json:"license,omitempty"`
	SourceURL      string   `json:"sourceUrl,omitempty"`
	SourceRevision string   `json:"sourceRevision,omitempty"`
	DeclaredTools  []string `json:"declaredTools,omitempty"`
}

var skillDeclaredTool = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,79}$`)

func skillMetadataText(value string, maximum int) bool {
	if len(value) > maximum || !utf8.ValidString(value) || value != strings.TrimSpace(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
			return false
		}
	}
	return true
}
func validProjectSkillMetadata(value *ProjectSkillMetadata) bool {
	if value == nil {
		return true
	}
	if !skillMetadataText(value.License, 128) || !skillMetadataText(value.SourceRevision, 200) || !skillMetadataText(value.SourceURL, 2048) || len(value.DeclaredTools) > 16 {
		return false
	}
	if value.SourceURL != "" {
		u, err := url.Parse(value.SourceURL)
		if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || strings.IndexFunc(value.SourceURL, unicode.IsSpace) >= 0 || strings.ContainsRune(value.SourceURL, '\\') {
			return false
		}
	}
	seen := map[string]bool{}
	for _, tool := range value.DeclaredTools {
		if !skillDeclaredTool.MatchString(tool) || seen[tool] {
			return false
		}
		seen[tool] = true
	}
	return true
}
func cloneProjectSkillMetadata(value *ProjectSkillMetadata) *ProjectSkillMetadata {
	if value == nil {
		return nil
	}
	out := *value
	out.DeclaredTools = append([]string(nil), value.DeclaredTools...)
	return &out
}
