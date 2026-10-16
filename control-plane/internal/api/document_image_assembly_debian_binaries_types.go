package api

// Pure observation schema only. No captured-run factory, ownership capability,
// adoption writer, route or full input-closure resolver is implemented here.
const assemblyDebianBinaryBytes int64 = 144257468
const assemblyDebianBinaryIndexBytes int64 = 9489432
const assemblyDebianBinaryReadLimit int64 = 320 << 20
const assemblyDebianBinaryBundleLimit int64 = 4 << 20
const assemblyDebianBinaryEvidenceLimit int64 = 16 << 20

type assemblyDebianBinaryExpectedTupleV1 struct {
	Ordinal             int      `json:"ordinal"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Architecture        string   `json:"architecture"`
	Filename            string   `json:"filename"`
	Bytes               int64    `json:"bytes"`
	SHA256              string   `json:"sha256"`
	AllowedCandidateIDs []string `json:"allowedCandidateIds"`
}

type assemblyDebianBinarySelectionV1 struct {
	Schema                    int                                   `json:"schema"`
	DependencyLockSHA256      string                                `json:"dependencyLockSha256"`
	AcquisitionManifestSHA256 string                                `json:"acquisitionManifestSha256"`
	CandidatePolicySHA256     string                                `json:"candidatePolicySha256"`
	Packages                  []assemblyDebianBinaryExpectedTupleV1 `json:"packages"`
}

// Null means absent. Values preserve case, operators and internal whitespace.
// Only physical continuation SP/TAB edges are removed and joined with one SP.
type assemblyDebianBinaryFieldsV1 struct {
	Package          *string `json:"package"`
	Version          *string `json:"version"`
	Architecture     *string `json:"architecture"`
	Depends          *string `json:"depends"`
	PreDepends       *string `json:"pre-depends"`
	Provides         *string `json:"provides"`
	Breaks           *string `json:"breaks"`
	Conflicts        *string `json:"conflicts"`
	Replaces         *string `json:"replaces"`
	Source           *string `json:"source"`
	BuiltUsing       *string `json:"built-using"`
	StaticBuiltUsing *string `json:"static-built-using"`
	MultiArch        *string `json:"multi-arch"`
}

type assemblyDebianBinaryIndexFieldsV1 struct {
	BindingOrdinals []int                        `json:"bindingOrdinals"` // positions0..11 in exact signed-index root
	Fields          assemblyDebianBinaryFieldsV1 `json:"fields"`
}

type assemblyDebianBinarySelectedFieldsV1 struct {
	Schema  int                                 `json:"schema"`
	Control assemblyDebianBinaryFieldsV1        `json:"control"`
	Indexes []assemblyDebianBinaryIndexFieldsV1 `json:"indexes"` // one per matched unique index, <=4
}

type assemblyDebianBinarySpanV1 struct {
	Bundle assemblyDebianObjectRef `json:"bundle"`
	Offset int64                   `json:"offset"`
	Bytes  int64                   `json:"bytes"`
	SHA256 string                  `json:"sha256"`
}

type assemblyDebianBinaryMatchV1 struct {
	IndexBindingOrdinals []int                      `json:"indexBindingOrdinals"`
	CompressedIndex      assemblyDebianObjectRef    `json:"compressedIndex"`
	DecodedIndexSHA256   string                     `json:"decodedIndexSha256"`
	DecodedIndexBytes    int64                      `json:"decodedIndexBytes"`
	ParagraphOffset      int64                      `json:"paragraphOffset"`
	Paragraph            assemblyDebianBinarySpanV1 `json:"paragraph"`
}

type assemblyDebianBinaryBindingV1 struct {
	LockOrdinal         int                           `json:"lockOrdinal"`
	Name                string                        `json:"name"`
	Version             string                        `json:"version"`
	Architecture        string                        `json:"architecture"`
	Filename            string                        `json:"filename"`
	DebObject           assemblyDebianObjectRef       `json:"debObject"`
	Matches             []assemblyDebianBinaryMatchV1 `json:"matches"`
	Control             assemblyDebianBinarySpanV1    `json:"control"`
	SelectedFieldClaims assemblyDebianBinarySpanV1    `json:"selectedFieldClaims"`
}

type assemblyDebianBinaryPartV1 struct {
	Schema               int                             `json:"schema"`
	RecordType           string                          `json:"recordType"`
	OperationID          string                          `json:"operationId"`
	Part                 int                             `json:"part"`
	SignedIndexes        assemblyCatalogRef              `json:"signedIndexes"`
	DependencyLockSHA256 string                          `json:"dependencyLockSha256"`
	FirstLockOrdinal     int                             `json:"firstLockOrdinal"`
	Bindings             []assemblyDebianBinaryBindingV1 `json:"bindings"`
}

type assemblyDebianBinaryPartRefV1 struct {
	Part             int                     `json:"part"`
	FirstLockOrdinal int                     `json:"firstLockOrdinal"`
	Count            int                     `json:"count"`
	Object           assemblyDebianObjectRef `json:"object"`
}

type assemblyDebianBinaryDecodedIndexV1 struct {
	Compressed    assemblyDebianObjectRef `json:"compressed"`
	DecodedSHA256 string                  `json:"decodedSha256"`
	DecodedBytes  int64                   `json:"decodedBytes"`
}

type assemblyDebianBinaryCountersV1 struct {
	CompressedReadBytes int64 `json:"compressedReadBytes"`
	ReadCalls           int   `json:"readCalls"`
	ExpandedBytes       int64 `json:"expandedBytes"`
	Paragraphs          int   `json:"paragraphs"`
}

type assemblyDebianBinariesV1 struct {
	Schema                    int                                  `json:"schema"`
	RecordType                string                               `json:"recordType"`
	ObservationState          string                               `json:"observationState"` // complete-observations-not-adopted
	SignedIndexes             assemblyCatalogRef                   `json:"signedIndexes"`
	DependencyLockSHA256      string                               `json:"dependencyLockSha256"`
	AcquisitionManifestSHA256 string                               `json:"acquisitionManifestSha256"`
	Producer                  assemblyDebianObjectRef              `json:"producer"`
	RetainedManifest          assemblyCatalogRef                   `json:"retainedManifest"`
	Parts                     []assemblyDebianBinaryPartRefV1      `json:"parts"`
	DecodedIndexes            []assemblyDebianBinaryDecodedIndexV1 `json:"decodedIndexes"`
	Counters                  assemblyDebianBinaryCountersV1       `json:"counters"`
	PackageCount              int                                  `json:"packageCount"`
	PackageBytes              int64                                `json:"packageBytes"`
	SourceArchiveBinding      string                               `json:"sourceArchiveBinding"`
	DependencyClosure         string                               `json:"dependencyClosure"`
	ExpectedInstalledFiles    string                               `json:"expectedInstalledFiles"`
	ObservedInstalledOutput   string                               `json:"observedInstalledOutput"`
	ServerEligibility         string                               `json:"serverEligibility"`
}

// Incomplete diagnostics have no root, producer capability or selectable ref.
type assemblyDebianBinaryIncompleteV1 struct {
	Schema       int    `json:"schema"`
	State        string `json:"state"` // incomplete-observations
	SeenOrdinals []int  `json:"seenOrdinals"`
	Reason       string `json:"reason"`
}

type assemblyDebianBinaryFrame struct {
	ordinal, kind, slot int
	span                assemblyDebianBinarySpanV1
	fields              map[string]string
	claims              *assemblyDebianBinarySelectedFieldsV1
}

// A pure bounded decoder result, not native completion/guard/authority proof.
type assemblyDebianBinaryBundle struct {
	object       assemblyDebianObjectRef
	first, count int
	frames       []assemblyDebianBinaryFrame
}
