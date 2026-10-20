package backend

// Closed lower-layer accounting only. These values do not create a native
// alias, prove a total handle census or authorize a child/producer/candidate.
const (
	DocumentImageAssemblyBinaryReadPurpose               = "debian-binary-byte-bindings-v1"
	DocumentImageAssemblyBinaryIndexLoanBytes      int64 = 19044408
	DocumentImageAssemblyBinaryIndexLoanCalls            = 768
	DocumentImageAssemblyBinarySourceLoanBytes     int64 = 393216
	DocumentImageAssemblyBinarySourceLoanCalls           = 16
	DocumentImageAssemblyBinaryLoanMaxReservations       = 256
)

// No paths/HANDLEs/PIDs on this request. The actual installer owner must derive
// ScopeDigest from its current typed run/profile/source/policy/CAS context. Hex
// syntax is containment only; this layer cannot establish current authority.
type DocumentImageAssemblyBinaryReadSubject struct {
	Role   string
	Bytes  int64
	Digest string
}

// Read-only shared identity of one already-held ordinary original. No HANDLE,
// path, cursor or ownership capability is exported by this description.
type DocumentImageAssemblyBinaryIdentity struct {
 FileType uint32 `json:"fileType"`
 FileAttributes uint32 `json:"fileAttributes"`
 VolumeSerial uint32 `json:"volumeSerial"`
 FileIndex uint64 `json:"fileIndex"`
 Bytes uint64 `json:"bytes"`
 CreationFileTime uint64 `json:"creationFileTime"`
 LastWriteFileTime uint64 `json:"lastWriteFileTime"`
 Links uint32 `json:"links"`
}
type DocumentImageAssemblyBinaryFingerprint struct {
 Slot uint16 `json:"slot"`
 Identity DocumentImageAssemblyBinaryIdentity `json:"identity"`
}

type DocumentImageAssemblyBinaryReadRequest struct {
	Purpose      string
	LoanID       string
	ScopeDigest  string
	Packages     [174]DocumentImageAssemblyBinaryReadSubject
	Indexes      [4]DocumentImageAssemblyBinaryReadSubject
	SmallSources [3]DocumentImageAssemblyBinaryReadSubject
}

// Informational reservation/retirement projection. There is deliberately no
// verified/alias-ready/census-success or process-settlement flag.
type DocumentImageAssemblyBinaryReadReservationState struct {
	Purpose, LoanID, ScopeDigest, Phase string
	IndexBytesCharged                   int64
	IndexCallsCharged                   int
	SourceBytesCharged                  int64
	SourceCallsCharged                  int
	ActiveGuardBorrows                  int
	NativeCensusState                   string // not-collected
	NativeAliasState                    string // not-created
}
