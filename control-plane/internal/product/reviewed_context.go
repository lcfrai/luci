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
	exports, err := journeyContextExports(state)
	if err != nil {
		return nil, err
	}
	files := make([]ReviewedContextFile, 0, len(exports)+2)
	hasBrief := false
	for _, item := range exports {
		hasBrief = hasBrief || item.Path == "LUCI.md"
		files = append(files, ReviewedContextFile{Path: item.Path, Data: item.Data})
	}
	if !hasBrief {
		files = append(files, ReviewedContextFile{Path: "LUCI.md", Data: renderBrief(state.Brief)})
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
