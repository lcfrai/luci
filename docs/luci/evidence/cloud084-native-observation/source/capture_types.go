package e2bbe

import (
	"regexp"
	"time"
)

const MaxCaptureArchiveBytes int64 = MaxTransferBytes + MaxTransferFiles*1024 + maxManifestBytes + 2048

const SourceSnapshotPolicy = "luci-task-source-v1"

// SourceSnapshot lists the exact source payload of an output archive. Paths
// are source-relative here; archive entries use output/source/<path>. The
// original input bundle remains the baseline, even if the task edits ignore
// files. These descriptions grant neither execution nor integration authority.
type SourceSnapshot struct {
	Version           int            `json:"version"`
	Policy            string         `json:"policy"`
	InputBundleSHA256 string         `json:"inputBundleSha256"`
	Files             []TransferFile `json:"files"`
}

type SourceChange struct {
	Path   string        `json:"path"`
	Kind   string        `json:"kind"` // added, modified or deleted
	Before *TransferFile `json:"before,omitempty"`
	After  *TransferFile `json:"after,omitempty"`
}

type SourceExclusion struct {
	Path   string `json:"path"`
	Reason string `json:"reason"` // fixed-new-path-exclusion
}

type SourceChanges struct {
	Version              int               `json:"version"`
	Policy               string            `json:"policy"`
	InputBundleSHA256    string            `json:"inputBundleSha256"`
	SourceManifestSHA256 string            `json:"sourceManifestSha256"`
	Changes              []SourceChange    `json:"changes"`
	Exclusions           []SourceExclusion `json:"exclusions"`
}

// CaptureIdentity is recorded first by the trusted root launch bootstrap and
// bound to the exact acknowledged envd process. A collector cannot introduce a
// new boot/cgroup identity at capture time. Validation checks equality/shape;
// it does not replace authenticated readback from a protected template path.
type CaptureIdentity struct {
	Version            int      `json:"version"`
	Identity           Identity `json:"identity"`
	RequestDigest      string   `json:"requestDigest"`
	SandboxID          string   `json:"sandboxId"`
	LaunchDigest       string   `json:"launchDigest"`
	ProcessPID         uint32   `json:"processPid"`
	InputBundleDigest  string   `json:"inputBundleDigest"`
	SourceDigest       string   `json:"sourceDigest"`
	ContextDigest      string   `json:"contextDigest"`
	BootID             string   `json:"bootId"`
	CgroupRelativePath string   `json:"cgroupRelativePath"`
	CgroupInode        uint64   `json:"cgroupInode"`
}

// CaptureReceipt describes a trusted producer's immutable archive after its
// dedicated task cgroup was actually observed frozen. There is deliberately no
// caller-settable Quiescent/Successful flag. The producer must keep the group
// frozen until exact deletion. Native process outcome is separate, as are
// independent checks, integration and provider billing.
type CaptureReceipt struct {
	Version              int             `json:"version"`
	CaptureID            string          `json:"captureId"`
	Identity             CaptureIdentity `json:"identity"`
	ArchiveSHA256        string          `json:"archiveSha256"`
	ArchiveBytes         int64           `json:"archiveBytes"`
	CapturedAt           time.Time       `json:"capturedAt"`
	SourceManifestSHA256 string          `json:"sourceManifestSha256"`
	ChangeManifestSHA256 string          `json:"changeManifestSha256"`
}

// NativeExitReceipt is independent of an envd Start/End stream or capture
// command. Only the authenticated sandboxd typed Exit frame for this launch
// may establish it. Output text containing a status sentence is not evidence.
type NativeExitReceipt struct {
	Version       int       `json:"version"`
	SandboxID     string    `json:"sandboxId"`
	RequestDigest string    `json:"requestDigest"`
	LaunchDigest  string    `json:"launchDigest"`
	ProcessPID    uint32    `json:"processPid"`
	FrameSHA256   string    `json:"frameSha256"`
	Reason        string    `json:"reason"`
	ObservedAt    time.Time `json:"observedAt"`
}

var bootIDPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)
var nativeExitReasonPattern = regexp.MustCompile(`^(exit status ([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])|signal: [a-z][a-z0-9 ]{0,63}( \(core dumped\))?)$`)

// ValidateCaptureIdentityShape checks only the guest protocol's structural
// requirements. The guest must separately bind it to its verified input and
// protected launch receipt. Controller callers must use the full record check
// below; valid shape alone is never evidence of ownership or acknowledgement.
func ValidateCaptureIdentityShape(id CaptureIdentity) error {
	if id.Version != 1 || !validID(id.Identity.OwnerID) || !validID(id.Identity.ProjectID) ||
		!validID(id.Identity.TaskID) || !validID(id.Identity.AttemptID) || !validID(id.Identity.OperationID) ||
		!validID(id.SandboxID) || !digestPattern.MatchString(id.RequestDigest) ||
		!digestPattern.MatchString(id.LaunchDigest) || !digestPattern.MatchString(id.InputBundleDigest) ||
		!digestPattern.MatchString(id.SourceDigest) || !digestPattern.MatchString(id.ContextDigest) ||
		id.ProcessPID == 0 || !bootIDPattern.MatchString(id.BootID) ||
		id.BootID == "00000000-0000-0000-0000-000000000000" || id.CgroupInode == 0 ||
		id.CgroupRelativePath != "luci-tasks/"+id.Identity.OperationID {
		return ErrConflict
	}
	return nil
}

// ValidateCaptureIdentity requires an already acknowledged launch. Lifecycle
// authority/current allocation state is rechecked by the operation performing
// I/O, not inferred from this pure metadata check (which also reads history).
func ValidateCaptureIdentity(id CaptureIdentity, r Record) error {
	if ValidateCaptureIdentityShape(id) != nil || !validRequest(r.Request) || r.RequestDigest != digest(r.Request) ||
		!r.LaunchRequested || !digestPattern.MatchString(r.LaunchDigest) || r.ProcessPID == 0 ||
		id.Identity != r.Request.Identity || id.RequestDigest != r.RequestDigest ||
		id.SandboxID != r.SandboxID || !validID(id.SandboxID) ||
		id.LaunchDigest != r.LaunchDigest || id.ProcessPID != r.ProcessPID ||
		id.InputBundleDigest != r.Request.BundleDigest || id.SourceDigest != r.Request.SourceDigest ||
		id.ContextDigest != r.Request.ContextDigest {
		return ErrConflict
	}
	return nil
}

// ValidateCaptureReceipt compares the producer identity to a previously
// authenticated immutable launch identity. Hash equality here is a contract,
// not verification of the corresponding archive or JSON metadata bytes.
func ValidateCaptureReceipt(receipt CaptureReceipt, launch CaptureIdentity, r Record) error {
	if ValidateCaptureIdentity(launch, r) != nil || receipt.Version != 1 ||
		!validID(receipt.CaptureID) || receipt.Identity != launch ||
		!digestPattern.MatchString(receipt.ArchiveSHA256) || receipt.ArchiveBytes <= 0 ||
		receipt.ArchiveBytes > MaxCaptureArchiveBytes ||
		receipt.CapturedAt.IsZero() || !digestPattern.MatchString(receipt.SourceManifestSHA256) ||
		!digestPattern.MatchString(receipt.ChangeManifestSHA256) {
		return ErrTransfer
	}
	return nil
}

func ValidateNativeExitReceipt(receipt NativeExitReceipt, r Record) error {
	if receipt.Version != 1 || !validRequest(r.Request) || r.RequestDigest != digest(r.Request) ||
		!r.LaunchRequested || !digestPattern.MatchString(r.LaunchDigest) || r.ProcessPID == 0 ||
		receipt.SandboxID != r.SandboxID || !validID(r.SandboxID) ||
		receipt.RequestDigest != r.RequestDigest || receipt.LaunchDigest != r.LaunchDigest ||
		receipt.ProcessPID != r.ProcessPID || receipt.ObservedAt.IsZero() ||
		!nativeExitReasonPattern.MatchString(receipt.Reason) ||
		receipt.FrameSHA256 != bytesDigest(append([]byte{0x03}, []byte(receipt.Reason)...)) {
		return ErrConflict
	}
	return nil
}
