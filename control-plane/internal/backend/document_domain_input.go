package backend

import (
	"context"
	"io"
)

// DomainInput describes an original, immutable inspection input. These fields
// are expected metadata supplied by a trusted caller, not proof or admission.
type DomainInput struct {
	Role string
	RelativePath string
	Bytes int64
	SHA256 string
}

const (
	DocumentDomainInputLimit = 512
	// Corresponding-source retention is separate from the assembly payload.
	// Large original archives are streamed; these limits do not authorize a
	// matching caller-buffer allocation or enlarge any existing image guard.
	DocumentDomainInputFileBytes int64 = 8 << 30
	DocumentDomainInputTotalBytes int64 = 16 << 30
	DocumentDomainInputReadAtBytes int64 = 128 << 20
	DocumentDomainInputBlockBytes = 64 << 10
	DocumentDomainInputReadCalls uint64 = 1048576
	DocumentDomainInputReadBytes uint64 = 64 << 30
	DocumentDomainInputAncestorLimit = 64
	// Includes ordinary parents, leaves, and the one outstanding read event.
	// This is a separate demand, never spare space in another owner's census.
	DocumentDomainInputHandleLimit = 1024
)

type DocumentDomainInputDemand struct {
	Inputs int
	OriginalHandles int
	DeclaredBytes int64
	ReadCalls uint64
	ReadBytes uint64
	BlockBytes int
	AcquisitionWorkers int
	RetirementWorkers int
	CancellationWorkers int
	CancellationCallbacks int
	ReadBufferBytes int
}

// DocumentDomainInputs owns every acquired original, including a partial open
// or cancelled read. ReaderAt copies from that original; it never exposes a
// handle or borrowed native buffer. A nil Close is the only complete retirement
// acknowledgement. An error retains this owner and its outstanding charge.
// Cancellation of the opening or any captured reader context permanently
// prevents further reads. Each declared role can capture exactly one ReaderAt.
type DocumentDomainInputs interface {
	Demand() DocumentDomainInputDemand
	ReaderAt(context.Context, string) (io.ReaderAt, error)
	CheckCurrent(context.Context) error
	Close() error
}
