package api

import (
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

const assemblyDebianIndexProfile = "luci-debian-index-byte-bindings-v1"
const assemblyDebianIndexBytes int64 = 19507444
const assemblyDebianIndexObjectLimit int64 = 16 << 20

type assemblyDebianIndexLocationV1 struct {
	ConfiguredURL       string `json:"configuredUrl"`
	ConfiguredByHashURL string `json:"configuredByHashUrl"`
}

type assemblyDebianIndexBindingV1 struct {
	CandidateID      string                            `json:"candidateId"`
	Era              string                            `json:"era"`
	ReleaseOrdinal   int                               `json:"releaseOrdinal"`
	IndexPath        string                            `json:"indexPath"`
	CompressedObject assemblyDebianObjectRef           `json:"compressedObject"`
	Location         assemblyDebianIndexLocationV1     `json:"location"`
	HistoricalAnchor *assemblyDebianHistoricalAnchorV1 `json:"historicalAnchor"`
}

type assemblyDebianIndexSelectionV1 struct {
	Schema         int                            `json:"schema"`
	Profile        string                         `json:"profile"`
	SourcePolicy   assemblyDebianObjectRef        `json:"sourcePolicy"`
	ProducerConfig assemblyDebianObjectRef        `json:"producerConfig"`
	Objects        []assemblyDebianObjectRef      `json:"objects"`
	Bindings       []assemblyDebianIndexBindingV1 `json:"bindings"`
}

type assemblyDebianIndexCaptureV1 struct {
	Profile          string                  `json:"profile"`
	Config           assemblyDebianObjectRef `json:"config"`
	SourceReceipt    assemblyDebianObjectRef `json:"sourceReceipt"`
	RetainedManifest assemblyCatalogRef      `json:"retainedManifest"`
	ObservedAt       int64                   `json:"observedAt"`
	Method           string                  `json:"method"`
}

type assemblyDebianSignedIndexBindingsV1 struct {
	Schema                    int                            `json:"schema"`
	RecordType                string                         `json:"recordType"`
	RootPolicy                assemblyCatalogRef             `json:"rootPolicy"`
	KeyObservations           assemblyCatalogRef             `json:"keyObservations"`
	ReleaseSignatures         assemblyCatalogRef             `json:"releaseSignatures"`
	DependencyLockSHA256      string                         `json:"dependencyLockSha256"`
	AcquisitionManifestSHA256 string                         `json:"acquisitionManifestSha256"`
	Capture                   assemblyDebianIndexCaptureV1   `json:"capture"`
	Objects                   []assemblyDebianObjectRef      `json:"objects"`
	Bindings                  []assemblyDebianIndexBindingV1 `json:"bindings"`
	PayloadDecode             string                         `json:"payloadDecode"`
	ParagraphCoverage         string                         `json:"paragraphCoverage"`
	ServerEligibility         string                         `json:"serverEligibility"`
}

// A future installer-owned acquisition bridge alone may mint this live value.
// No minting function, public input, path adoption or restart reconstruction is
// provided here. completion comes from its actual original-process owner, not
// decoded JSON. The guard owns eight index files plus its exact receipt before
// any read; each logical role is "xz-index:"+the fixed lower-case SHA256.
type assemblyDebianCapturedIndexes struct {
	mu         sync.Mutex
	owner      *assemblyInputOwner
	first3     *assemblyDebianDomainCandidate
	id         string
	guard      backend.DocumentImageAssemblyEvidenceGuard
	completion *assemblyDebianOriginalCompletion
	receipt    assemblyDebianObjectRef
	selection  assemblyDebianIndexSelectionV1
	phase      string
}

type assemblyDebianIndexRetainedManifestV1 struct {
	Schema      int                       `json:"schema"`
	RunID       string                    `json:"runId"`
	ExpectedCAS assemblyCatalogCAS        `json:"expectedCas"`
	Source      assemblyCatalogSource     `json:"source"`
	Policy      assemblyCatalogPolicy     `json:"policy"`
	First3      []assemblyCatalogRef      `json:"first3"`
	Config      assemblyDebianObjectRef   `json:"config"`
	Receipt     assemblyDebianObjectRef   `json:"receipt"`
	Objects     []assemblyDebianObjectRef `json:"objects"`
}

// One owner after transfer. No payload map: each <=16MiB index is read and
// discarded sequentially. Both read rounds share the same guard lifetime budget.
type assemblyDebianIndexCandidate struct {
	mu                                                         sync.Mutex
	owner                                                      *assemblyInputOwner
	first3                                                     *assemblyDebianDomainCandidate // immutable typed evidence only after move
	capture                                                    *assemblyDebianCapturedIndexes
	output, inputs, indexes                                    backend.DocumentImageAssemblyEvidenceGuard
	expected                                                   assemblyCatalogCAS
	record                                                     assemblyDebianSignedIndexBindingsV1
	manifest                                                   assemblyDebianIndexRetainedManifestV1
	config, receipt, canonical, manifestBytes                  []byte
	ref, manifestRef                                           assemblyCatalogRef
	targetCAS                                                  assemblyCatalogCAS
	targetIndex                                                assemblyCatalogIndex
	retiring, adopting, publicationRetired, publicationUnknown bool
	publication                                                *assemblyDebianPublicationPermit
}
