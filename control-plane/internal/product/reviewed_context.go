package product

import (
	"encoding/json"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// ReviewedContextFile contains generated project context, never private review
// receipts, credentials, actors or historical conversation. The API places
// these bytes in its session-owned private read-only mount, not in the checkout.
type ReviewedContextFile struct {
	Path string
	Data []byte
}

type ReviewedContextCapture struct {
	Binding *registry.ImplementationContext
	Files   []ReviewedContextFile
}

// Current canonical state is bounded to 8 MiB. The bundle also contains its
// human-readable renderings; keep a separate finite bound on that expansion.
const ReviewedContextBytesLimit = 32 << 20

func captureReviewedContext(state journeyState, binding *registry.ImplementationContext) (*ReviewedContextCapture, error) {
	if binding == nil {
		return nil, errors.New("The reviewed context has no selected assignment.")
	}
	// Binding and documents must come from one reviewed snapshot. A caller that
	// reread state after reserving work cannot attach a newer definition/design
	// to an older task identity, even when that task's title did not change.
	current, err := implementationSnapshot(state, *ImplementationContextSelection(binding), binding.WorktreeID)
	if err != nil {
		return nil, err
	}
	selected := binding.Clone()
	selected.SourceCommit, selected.SourceDigest, selected.SourceDirty = "", "", false
	if implementationDigest(current) != implementationDigest(selected) {
		return nil, errors.New("The reviewed documents changed after this assignment was selected. Review the current inputs before starting another run.")
	}
	taskBrief, err := renderReviewedTaskBrief(state, binding)
	if err != nil {
		return nil, err
	}
	exports, err := journeyContextExports(state)
	if err != nil {
		return nil, err
	}
	files := make([]ReviewedContextFile, 0, len(exports)+2)
	hasBrief := false
	for _, item := range exports {
		hasBrief = hasBrief || item.Path == "LUCI.md"
		if item.Path == "LUCI.md" {
			// The working checkout's export remains the ordinary project brief.
			// Only this new immutable session capture gets its selected contract.
			item.Data = append(append([]byte{}, item.Data...), taskBrief...)
		}
		files = append(files, ReviewedContextFile{Path: item.Path, Data: item.Data})
	}
	if !hasBrief {
		files = append(files, ReviewedContextFile{Path: "LUCI.md", Data: append(renderBrief(state.Brief), taskBrief...)})
	}
	brief, err := json.MarshalIndent(state.Brief, "", "  ")
	if err != nil {
		return nil, err
	}
	files = append(files, ReviewedContextFile{Path: ".luci/brief.json", Data: append(brief, '\n')})
	total := 0
	for _, item := range files {
		total += len(item.Data)
		if total > ReviewedContextBytesLimit {
			return nil, errors.New("The reviewed context is too large to deliver safely. Reduce the saved context before starting a new reviewed run.")
		}
	}
	return &ReviewedContextCapture{Binding: binding, Files: files}, nil
}
