package e2bbe

import (
	"path"
	"reflect"
	"strings"
)

// ValidateReturnedSource compares the trusted collector's source inventory and
// change list with both complete transfer manifests. It does not approve the
// edits or write them into a project. Callers must still bind input to the
// current reviewed Git/context revision and run independent review and checks.
func ValidateReturnedSource(req Request, input, output TransferManifest, receipt CaptureReceipt, sourceBytes, changeBytes []byte) (SourceChanges, error) {
	var changes SourceChanges
	inputBinding := TransferBinding{Identity: req.Identity, SourceDigest: req.SourceDigest, ContextDigest: req.ContextDigest, Direction: "input"}
	outputBinding := inputBinding
	outputBinding.Direction = "output"
	if !validRequest(req) || input.Binding != inputBinding || output.Binding != outputBinding ||
		validateManifest(input) != nil || validateManifest(output) != nil ||
		receipt.ArchiveSHA256 == "" || len(sourceBytes) == 0 || len(changeBytes) == 0 ||
		len(sourceBytes)+len(changeBytes) > maxManifestBytes ||
		bytesDigest(sourceBytes) != receipt.SourceManifestSHA256 || bytesDigest(changeBytes) != receipt.ChangeManifestSHA256 {
		return changes, ErrTransfer
	}
	var source SourceSnapshot
	if decodeStrict(sourceBytes, &source) != nil || decodeStrict(changeBytes, &changes) != nil ||
		source.Version != 1 || changes.Version != 1 || source.Policy != SourceSnapshotPolicy || changes.Policy != SourceSnapshotPolicy ||
		source.InputBundleSHA256 != req.BundleDigest || changes.InputBundleSHA256 != req.BundleDigest ||
		changes.SourceManifestSHA256 != receipt.SourceManifestSHA256 {
		return SourceChanges{}, ErrTransfer
	}
	files := make(map[string]TransferFile, len(output.Files))
	for _, file := range output.Files {
		files[file.Path] = file
	}
	if files["output/source-manifest.json"].SHA256 != receipt.SourceManifestSHA256 ||
		files["output/changes.json"].SHA256 != receipt.ChangeManifestSHA256 ||
		files["output/source-manifest.json"].Size != int64(len(sourceBytes)) ||
		files["output/changes.json"].Size != int64(len(changeBytes)) {
		return SourceChanges{}, ErrTransfer
	}
	before := make(map[string]TransferFile, len(input.Files))
	for _, file := range input.Files {
		if strings.HasPrefix(file.Path, "source/") {
			file.Path = strings.TrimPrefix(file.Path, "source/")
			before[file.Path] = file
		}
	}
	after := make(map[string]TransferFile, len(source.Files))
	previous := ""
	for _, file := range source.Files {
		if file.Path <= previous || !validTransferPath("source/"+file.Path, "input") {
			return SourceChanges{}, ErrTransfer
		}
		previous = file.Path
		archived, exists := files["output/source/"+file.Path]
		if !exists || archived.Size != file.Size || archived.SHA256 != file.SHA256 || archived.Mode != file.Mode {
			return SourceChanges{}, ErrTransfer
		}
		after[file.Path] = file
	}
	if len(output.Files) != len(source.Files)+2 {
		return SourceChanges{}, ErrTransfer
	}
	remaining := make(map[string]TransferFile, len(before))
	for name, file := range before {
		remaining[name] = file
	}
	expected := make(map[string]SourceChange)
	for name, file := range after {
		old, exists := remaining[name]
		current := file
		if !exists {
			expected[name] = SourceChange{Path: name, Kind: "added", After: &current}
		} else if old != file {
			prior := old
			expected[name] = SourceChange{Path: name, Kind: "modified", Before: &prior, After: &current}
		}
		delete(remaining, name)
	}
	for name, file := range remaining {
		prior := file
		expected[name] = SourceChange{Path: name, Kind: "deleted", Before: &prior}
	}
	if len(changes.Changes) != len(expected) || len(changes.Exclusions) > MaxTransferFiles {
		return SourceChanges{}, ErrTransfer
	}
	previous = ""
	for _, change := range changes.Changes {
		want, exists := expected[change.Path]
		if !exists || change.Path <= previous || !reflect.DeepEqual(change, want) {
			return SourceChanges{}, ErrTransfer
		}
		previous = change.Path
	}
	previous = ""
	for _, excluded := range changes.Exclusions {
		if excluded.Path <= previous || excluded.Reason != "fixed-new-path-exclusion" ||
			!validSourceExclusionPath(excluded.Path) ||
			!captureExcludedSourcePath(excluded.Path) {
			return SourceChanges{}, ErrTransfer
		}
		for name := range before {
			if name == excluded.Path || strings.HasPrefix(name, excluded.Path+"/") {
				return SourceChanges{}, ErrTransfer
			}
		}
		for name := range after {
			if name == excluded.Path || strings.HasPrefix(name, excluded.Path+"/") {
				return SourceChanges{}, ErrTransfer
			}
		}
		previous = excluded.Path
	}
	return changes, nil
}

// This mirrors only the fixed new-path exclusions used by the trusted guest
// collector. Existing input files are checked above and may never be hidden.
func captureExcludedSourcePath(name string) bool {
	for _, part := range strings.Split(name, "/") {
		switch part {
		case ".git", "node_modules", ".venv", "venv", "__pycache__", ".next", ".nuxt", ".cache", ".pytest_cache", "coverage":
			return true
		}
	}
	base := path.Base(name)
	return base == ".env" || strings.HasPrefix(base, ".env.") && base != ".env.example" && base != ".env.sample" || base == "id_rsa" || base == "id_ed25519"
}

func validSourceExclusionPath(name string) bool {
	if name == "" || len(name) > 230 || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00*?\"<>|") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." || strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") {
			return false
		}
		for _, c := range part {
			if c < 32 || c > 126 {
				return false
			}
		}
	}
	return true
}
