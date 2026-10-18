//go:build windows

package dockerbe

import (
	"context"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func validateAssemblyNamespaceDirectory(directory string) error {
	volume := filepath.VolumeName(directory)
	if !utf8.ValidString(directory) || len(directory) > 32760 || len(utf16.Encode([]rune(directory))) > 32760 || !filepath.IsAbs(directory) || filepath.Clean(directory) != directory || len(volume) != 2 || volume[1] != ':' || strings.Contains(directory[len(volume):], ":") || strings.ContainsAny(directory, "\x00\r\n") {
		return backend.ErrUnsupported
	}
	ancestors := 0
	for parent := filepath.Dir(directory); ; parent = filepath.Dir(parent) {
		ancestors++
		if ancestors > 64 { return backend.ErrUnsupported }
		if parent == filepath.Dir(parent) { break }
	}
	return nil
}

func collectAssemblyNamespace(ctx context.Context, g *assemblySourceGuard, directory string) error {
	collector := newAssemblySourceCollectorForGuard(ctx, g)
	if err := collector.parents(directory); err != nil { return err }
	if err := collector.open(directory, true, 0); err != nil { return err }
	if err := ctx.Err(); err != nil { return err }
	// Actual collector return publishes readiness; partial handles remain on g.
	g.ready = true
	return nil
}
