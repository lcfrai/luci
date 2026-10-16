// Bounded private three-domain metadata. These types do not establish observed
// producer facts or accepted closure. No public registration or adoption exists.
package api

// Object names/paths never appear on this wire. The private owned producer maps
// exact run manifest entries to retained objects, verifies bytes under original
// held guards, and resolves these closed-role addresses inside the existing owner.
type assemblyDebianObjectRef struct {
	Role string `json:"role"` // policy-source, public-key, keyring, key-listing,
	// signed-release, signed-plaintext, signature-status, producer-config,
	// producer-source-receipt, producer-output, inner-receipt, outer-receipt.
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

type assemblyDebianRootV1 struct {
	ID                       string                  `json:"id"`                 // exact three root IDs from selected13bd policy
	PrimaryFingerprint       string                  `json:"primaryFingerprint"` // uppercase40hex
	SigningSubkeyFingerprint *string                 `json:"signingSubkeyFingerprint"`
	PublicKey                assemblyDebianObjectRef `json:"publicKey"`
	OfficialOrigin           string                  `json:"officialOrigin"`      // attribution only, never fetched
	AnnouncedExpiryDate      *string                 `json:"announcedExpiryDate"` // informational
}

type assemblyDebianReleaseSelectionV1 struct {
	CandidateID string `json:"candidateId"`
	Role        string `json:"role"` // current-updates, current-security,
	// exact-point-anchor, historical-reproducible-anchor; never inferred by URL
	SignedRelease       assemblyDebianObjectRef            `json:"signedRelease"`
	AllowedRootIDs      []string                           `json:"allowedRootIds"` // sorted unique subset of3
	Origin              string                             `json:"origin"`
	Label               string                             `json:"label"`
	Suite               string                             `json:"suite"`
	Codename            string                             `json:"codename"`
	ExpectedReleaseDate int64                              `json:"expectedReleaseDate"` // signed Unix seconds
	ExpectedValidUntil  *int64                             `json:"expectedValidUntil"`  // absent is not infinity
	HistoricalTimestamp *string                            `json:"historicalTimestamp"` // exact20260918T000000Z
	SignedIndexes       []assemblyDebianSignedIndexClaimV1 `json:"signedIndexes"`
}

type assemblyDebianRootPolicyV1 struct {
	Schema                      int                                `json:"schema"`         //1
	RecordType                  string                             `json:"recordType"`     //luci-debian-official-root-policy-v1
	SourcePolicy                assemblyDebianObjectRef            `json:"sourcePolicy"`   //exact13bd bytes
	ProducerConfig              assemblyDebianObjectRef            `json:"producerConfig"` //exactv4 policy binding
	HistoricalAnchor            assemblyDebianHistoricalAnchorV1   `json:"historicalAnchor"`
	PolicyRevision              string                             `json:"policyRevision"` //current installer-selected revision
	DependencyLockSHA256        string                             `json:"dependencyLockSha256"`
	AcquisitionManifestSHA256   string                             `json:"acquisitionManifestSha256"`
	Roots                       []assemblyDebianRootV1             `json:"roots"`                       //exact3
	Releases                    []assemblyDebianReleaseSelectionV1 `json:"releases"`                    //exact6logical/5objects
	FutureSkewSeconds           int64                              `json:"futureSkewSeconds"`           //300, from frozen projection
	CurrentReleaseMaxAgeSeconds int64                              `json:"currentReleaseMaxAgeSeconds"` //172800
	RevocationKnowledge         string                             `json:"revocationKnowledge"`         //pinned-snapshot-only
	KeyObservationMaxAgeSeconds int64                              `json:"keyObservationMaxAgeSeconds"`
	// Explicit normative value86400, not an observed global revocation fact.
	// This does not assert current global revocation knowledge or latest release.
}

type assemblyDebianProducerV1 struct {
	Profile          string                  `json:"profile"` //luci-retained-debian-signature-input-evidence-v2
	SourceReceipt    assemblyDebianObjectRef `json:"sourceReceipt"`
	Config           assemblyDebianObjectRef `json:"config"`
	TypedOutput      assemblyDebianObjectRef `json:"typedOutput"`
	InnerReceipt     assemblyDebianObjectRef `json:"innerReceipt"`
	OuterReceipt     assemblyDebianObjectRef `json:"outerReceipt"`
	ProjectionSHA256 string                  `json:"projectionSha256"`
	GPGSHA256        string                  `json:"gpgSha256"`
	GPGVSHA256       string                  `json:"gpgvSha256"`
	// Source receipt also binds exact supervisor/outer/pinned DLL set. Actual
	// nine child identities/exit codes/Job+handle+PDH closure are validated from
	// owned receipts; these digest strings alone do not assert execution.
}

type assemblyDebianKeyV1 struct {
	Fingerprint        string `json:"fingerprint"`
	PrimaryFingerprint string `json:"primaryFingerprint"`
	Kind               string `json:"kind"` //pub or sub
	Created            int64  `json:"created"`
	Expires            *int64 `json:"expires"` //native0 becomes explicit absence
	Algorithm          int    `json:"algorithm"`
	Bits               int    `json:"bits"`
	Capabilities       string `json:"capabilities"`
	SnapshotValidity   string `json:"snapshotValidity"`
}

type assemblyDebianKeySnapshotV1 struct {
	RootID         string                  `json:"rootId"`
	PublicKey      assemblyDebianObjectRef `json:"publicKey"`
	DerivedKeyring assemblyDebianObjectRef `json:"derivedKeyring"`
}

type assemblyDebianKeyObservationsV1 struct {
	Schema            int                           `json:"schema"`     //1
	RecordType        string                        `json:"recordType"` //luci-debian-key-observations-v1
	RootPolicy        assemblyCatalogRef            `json:"rootPolicy"`
	Producer          assemblyDebianProducerV1      `json:"producer"`
	ObservedAt        int64                         `json:"observedAt"` //actual producer keyObservation.observedUtc
	KeyListing        assemblyDebianObjectRef       `json:"keyListing"`
	Snapshots         []assemblyDebianKeySnapshotV1 `json:"snapshots"`         //exact3
	Keys              []assemblyDebianKeyV1         `json:"keys"`              //sorted unique fingerprint, max32
	UnseenRevocations string                        `json:"unseenRevocations"` //not-established
}

type assemblyDebianValidSignatureV1 struct {
	Signer        string `json:"signer"`
	Primary       string `json:"primary"`
	Created       int64  `json:"created"`
	Expires       *int64 `json:"expires"`
	HashAlgorithm int    `json:"hashAlgorithm"` //8,9,10 only
}

type assemblyDebianUnverifiedExtraV1 struct {
	KeyID     string `json:"keyId"`     //uppercase16hex; NOT an accepted signer
	ErrorCode int    `json:"errorCode"` //exact9 with matched ERRSIG/NO_PUBKEY
}

type assemblyDebianSignatureDispositionV1 struct {
	Kind            string                           `json:"kind"` //eligible-valid-signature or unverified-extra-signature
	Valid           *assemblyDebianValidSignatureV1  `json:"valid,omitempty"`
	UnverifiedExtra *assemblyDebianUnverifiedExtraV1 `json:"unverifiedExtra,omitempty"`
	// Exactly one matching arm; extras never supply authority/quorum.
}

type assemblyDebianSignedIndexClaimV1 struct {
	Path   string `json:"path"` //only main/binary-amd64/Packages.xz or main/source/Sources.xz
	Bytes  int64  `json:"bytes"`
	SHA256 string `json:"sha256"`
	// A signed advertised tuple only. No acquired/index paragraph coverage claim.
}

type assemblyDebianLogicalReleaseV1 struct {
	CandidateID                 string                             `json:"candidateId"`
	ReleaseDate                 int64                              `json:"releaseDate"`
	ValidUntil                  *int64                             `json:"validUntil"`
	ProducerFreshness           string                             `json:"producerFreshness"` //attributed, non-authoritative
	ProducerHistoricalTimestamp *string                            `json:"producerHistoricalTimestamp"`
	Era                         string                             `json:"era"` //current or historical-base-anchor
	ObservedAt                  int64                              `json:"observedAt"`
	SignatureState              string                             `json:"signatureState"`
	HistoricalAnchor            *assemblyDebianHistoricalAnchorV1  `json:"historicalAnchor"`
	UpstreamExpiryState         string                             `json:"upstreamExpiryState"`
	HistoricalExpiryState       *string                            `json:"historicalExpiryState"`
	KeyRevocationKnowledge      string                             `json:"keyRevocationKnowledge"`
	ServerEligibility           string                             `json:"serverEligibility"`
	SignedIndexes               []assemblyDebianSignedIndexClaimV1 `json:"signedIndexes"` //exact2
}

// Exact v4 LOCAL117 object roles, never an ambiguous base digest or an inferred
// debuerreotype archive size. Snapshot provenance is metadata, not OCI acceptance.
type assemblyDebianHistoricalAnchorV1 struct {
	ID                               string                  `json:"id"`
	SnapshotTimestamp                string                  `json:"snapshotTimestamp"`
	SnapshotSourceRef                assemblyDebianObjectRef `json:"snapshotSourceRef"`
	TrustPolicySHA256                string                  `json:"trustPolicySha256"`
	DependencyLockSHA256             string                  `json:"dependencyLockSha256"`
	BootstrapConfigDigest            string                  `json:"bootstrapConfigDigest"`
	CompressedLayerRef               assemblyDebianObjectRef `json:"compressedLayerRef"`
	DebuerreotypeRootfsArchiveSHA256 string                  `json:"debuerreotypeRootfsArchiveSha256"`
}

type assemblyDebianReleaseVerificationV1 struct {
	SignedRelease   assemblyDebianObjectRef                `json:"signedRelease"`
	SignedPlaintext assemblyDebianObjectRef                `json:"signedPlaintext"`
	MachineStatus   assemblyDebianObjectRef                `json:"machineStatus"`
	ChildIndex      int                                    `json:"childIndex"`    //exact4..8 in fixed nine-child batch
	ExitCode        int                                    `json:"exitCode"`      //0 or2 only, exact status-disposition relation
	ObservedStart   int64                                  `json:"observedStart"` //actual verification-child start, rounded down
	ObservedEnd     int64                                  `json:"observedEnd"`   //actual producer completion upper bound, rounded up
	Dispositions    []assemblyDebianSignatureDispositionV1 `json:"dispositions"`  //1..16
	Logical         []assemblyDebianLogicalReleaseV1       `json:"logical"`       //1..2, total6
}

type assemblyDebianReleaseSignaturesV1 struct {
	Schema          int                                   `json:"schema"`     //1
	RecordType      string                                `json:"recordType"` //luci-debian-release-signatures-v1
	RootPolicy      assemblyCatalogRef                    `json:"rootPolicy"`
	KeyObservations assemblyCatalogRef                    `json:"keyObservations"`
	Producer        assemblyDebianProducerV1              `json:"producer"`
	Releases        []assemblyDebianReleaseVerificationV1 `json:"releases"` //exact5unique
}

// Future private producer-specific constructor seam, NOT IMPLEMENTED:
// captureDebianSignatureRun consumes only an actual exact owned run whose source,
// config, children, output objects and positive cleanup have been validated.
// newAssemblyDebianDomains(owner,currentInstaller,expectedCAS,selectedPolicy,
//                         capturedOwnedSignatureRun,borrowedCurrentObjectGuard)
// validates/normalizes these three records and returns private domain evidence.
// It cannot construct a full assemblyAcceptedInputCandidate or publish an
// accepted InputClosureRef while the remaining nine domains are absent.
// expectedCAS is a private transaction precondition, not a field hashed into a
// policy/record that the same next catalog head selects (no self-reference).
