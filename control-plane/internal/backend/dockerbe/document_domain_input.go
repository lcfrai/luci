package dockerbe

import (
	"context"
	"encoding/hex"
	"path"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type domainInputPlan struct {
	root string
	inputs []backend.DomainInput
	demand backend.DocumentDomainInputDemand
}

// PlanDocumentDomainInputs is metadata-only. The caller must reserve this
// separate finite demand before Open; this function does not mint a reservation.
func PlanDocumentDomainInputs(root string, inputs []backend.DomainInput) (backend.DocumentDomainInputDemand, error) {
	p, err := planDocumentDomainInputs(root, inputs)
	return p.demand, err
}

func planDocumentDomainInputs(root string, inputs []backend.DomainInput) (domainInputPlan, error) {
	p := domainInputPlan{}
	if root == "" || len(root)>32760 || !filepath.IsAbs(root) || filepath.Clean(root) != root || !utf8.ValidString(root) || strings.ContainsAny(root, "\x00\r\n") || len(inputs) == 0 || len(inputs) > backend.DocumentDomainInputLimit {
		return p, backend.ErrUnsupported
	}
	paths, roles, originals := map[string]bool{}, map[string]bool{}, map[string]bool{}
	addOriginal:=func(value string)bool{key:=strings.ToLower(value);if originals[key]{return true};if len(originals)+1>=backend.DocumentDomainInputHandleLimit{return false};originals[key]=true;return true}
	var total int64
	for _, input := range inputs {
		if input.Role == "" || len(input.Role) > 128 || !utf8.ValidString(input.Role) || strings.IndexFunc(input.Role, unicode.IsControl) >= 0 || roles[input.Role] || !domainRelativePath(input.RelativePath) || input.Bytes < 0 || input.Bytes > backend.DocumentDomainInputFileBytes || len(input.SHA256) != 64 || strings.ToLower(input.SHA256) != input.SHA256 {
			return p, backend.ErrUnsupported
		}
		if _, err := hex.DecodeString(input.SHA256); err != nil { return p, backend.ErrUnsupported }
		key := strings.ToLower(input.RelativePath)
		if paths[key] || total > backend.DocumentDomainInputTotalBytes-input.Bytes { return p, backend.ErrUnsupported }
		roles[input.Role], paths[key] = true, true
		total += input.Bytes
		full := filepath.Join(root, filepath.FromSlash(input.RelativePath))
		rel, err := filepath.Rel(root, full)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) { return p, backend.ErrUnsupported }
		if !addOriginal(full){return p,backend.ErrUnsupported}
		depth := 0
		for dir := filepath.Dir(full); ; dir = filepath.Dir(dir) {
			depth++
			if depth > backend.DocumentDomainInputAncestorLimit { return p, backend.ErrUnsupported }
			if !addOriginal(dir){return p,backend.ErrUnsupported}
			if filepath.Dir(dir) == dir { break }
		}
	}
	// One event is owned for the sole outstanding native read, including late IO.
	if len(originals)+1 > backend.DocumentDomainInputHandleLimit { return p, backend.ErrUnsupported }
	p.root, p.inputs = root, append([]backend.DomainInput(nil), inputs...)
	p.demand = backend.DocumentDomainInputDemand{Inputs:len(inputs), OriginalHandles:len(originals)+1, DeclaredBytes:total, ReadCalls:backend.DocumentDomainInputReadCalls, ReadBytes:backend.DocumentDomainInputReadBytes, BlockBytes:backend.DocumentDomainInputBlockBytes, AcquisitionWorkers:1, RetirementWorkers:1, CancellationWorkers:1, CancellationCallbacks:len(inputs)+2, ReadBufferBytes:2*backend.DocumentDomainInputBlockBytes}
	return p, nil
}

func domainRelativePath(value string) bool {
	if value == "" || len(value) > 4096 || !utf8.ValidString(value) || path.Clean(value) != value || path.IsAbs(value) || strings.ContainsAny(value, "\\:\x00<>\"|?*") || strings.IndexFunc(value, unicode.IsControl) >= 0 { return false }
	for _, part := range strings.Split(value, "/") {
		if part == "" || part == "." || part == ".." || strings.TrimRight(part, ". ") != part { return false }
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base,"COM") || strings.HasPrefix(base,"LPT")) && base[3] >= '0' && base[3] <= '9' { return false }
	}
	return true
}

// Open returns a partial owner with an error whenever acquisition began. Its
// opening context must have a deadline and must remain live for all inspection.
// Callers must retain the owner on any failed Close; no admission is supplied.
func OpenDocumentDomainInputs(ctx context.Context, root string, inputs []backend.DomainInput) (backend.DocumentDomainInputs, error) {
	if ctx == nil { return nil, backend.ErrUnsupported }
	if _, ok := ctx.Deadline(); !ok { return nil, backend.ErrUnsupported }
	if err := ctx.Err(); err != nil { return nil, err }
	p, err := planDocumentDomainInputs(root, inputs)
	if err != nil { return nil, err }
	return openDocumentDomainInputs(ctx, p)
}
