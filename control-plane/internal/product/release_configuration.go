package product

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
)

var previewEnvironmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)

// Docker env-file records are literal NAME=VALUE lines, never shell syntax.
// Defend here as well as at configuration save so corrupted state fails closed.
func previewEnvironmentContent(values map[string]string) (string, error) {
	names := make([]string, 0, len(values))
	for name, value := range values {
		if !previewEnvironmentName.MatchString(name) || strings.ContainsAny(value, "\x00\r\n") {
			return "", errors.New("Application configuration must use environment names and single-line values without NUL.")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var content strings.Builder
	for _, name := range names {
		content.WriteString(name)
		content.WriteByte('=')
		content.WriteString(values[name])
		content.WriteByte('\n')
		if content.Len() > 512<<10 {
			return "", errors.New("Application runtime configuration exceeds 512 KiB.")
		}
	}
	return content.String(), nil
}

func (s *Server) writePreviewEnvironment(values map[string]string) (string, error) {
	content, err := previewEnvironmentContent(values)
	if err != nil || content == "" {
		return "", err
	}
	file, err := os.CreateTemp(s.stateDir, "preview-env-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	keep := false
	defer func() {
		_ = file.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	// Set POSIX mode / Windows owner ACL before writing any secret bytes.
	if err := restrictPrivateFile(path); err != nil {
		return "", err
	}
	if _, err := file.WriteString(content); err != nil {
		return "", err
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	keep = true
	return path, nil
}

func redactPreviewConfiguration(message string, values map[string]string) string {
	secrets := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			secrets = append(secrets, value)
		}
	}
	sort.Slice(secrets, func(i, j int) bool { return len(secrets[i]) > len(secrets[j]) })
	for _, value := range secrets {
		message = strings.ReplaceAll(message, value, "[redacted]")
	}
	return message
}
