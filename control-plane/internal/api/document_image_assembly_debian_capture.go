package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"sort"
	"strconv"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const assemblyDebianSourceReceiptSHA = "13a80135f8b3b6392d8dc69df5f441a34eae0aa9592cc94dc6710ef2e061d1e0"
const assemblyDebianInnerSHA = "a32f91379525c0a3732c30dae9d1f4a73c3e710949589b6cea22be3d1f2d1553"

var errAssemblyDebianCapture = errors.New("owned Debian producer capture incomplete or unavailable")

// A concrete original-process-owner completion, NOT a decoded receipt or a
// caller's Verified flag. The product arm is populated only by the concrete
// installer-owned Windows original-handle completion path. Its packaged input
// family remains unavailable; legacy external-v4 constraints are unchanged.
// Tests can construct synthetic owners in *_test.go only.
type assemblyDebianOriginalCompletion struct {
	product *debianProductOriginalCompletion // closed Go-owner arm; no API-process exit claim
	runID string
	outerSHA string
	innerReceiptSHA, outerReceiptSHA, finalProjectionSHA string
	started, completed float64
	exitCode int
	originalHandleOwned, exitConfirmed, capturesSettled, watchdogSettled bool
	observerSettled, publicationSettled, handlesSettled bool
	innerCommand []string
	unknown bool
}

// The existing installer owns this live capability and every guard until an
// explicit positive Close. It is neither serializable nor reconstructible from
// producer-runs files after restart. This file starts no process and accepts no
// audit-artifact path, command, grant or completion callback.
type assemblyDebianOwnedRun struct {
	product *debianProductProofBinding // nil only for preserved legacy external-v4/test arm
	mu sync.Mutex
	owner *assemblyInputOwner
	id string
	actor management.ExecutionActorRef
	expected assemblyCatalogCAS
	source assemblyCatalogSource
	policy assemblyCatalogPolicy
	completion *assemblyDebianOriginalCompletion
	guard backend.DocumentImageAssemblyEvidenceGuard
	inputGuard backend.DocumentImageAssemblyEvidenceGuard
	commands [9][]string // installer-selected exact argv, captured by real launch owner
	objects map[string]assemblyDebianObjectRef // fixed owner-internal slots, not paths
	phase string
}

type assemblyDebianCapturedRun struct {
	run *assemblyDebianOwnedRun
	raw map[string][]byte
	inner assemblyDebianNativeReceipt
	output assemblyDebianRawOutput
}

func (r *assemblyDebianOwnedRun) current(ctx context.Context) error {
	if r == nil { return errAssemblyDebianCapture }
	r.mu.Lock()
	owner, guard, inputs, phase := r.owner, r.guard, r.inputGuard, r.phase
	r.mu.Unlock()
	if owner == nil || guard == nil || inputs == nil || phase == "retiring" || phase == "closed" || phase == "candidate" || ctx.Err() != nil { return errAssemblyDebianCapture }
	if err := guard.CheckCurrent(ctx); err != nil { return err }
	if err := inputs.CheckCurrent(ctx); err != nil { return err }
	if err := owner.authorize(ctx, r.actor); err != nil { return err }
	if r.product!=nil{if err:=r.product.current(ctx);err!=nil{return err}}
	r.mu.Lock(); retired := r.phase == "retiring" || r.phase == "closed" || r.phase == "candidate"; r.mu.Unlock()
	if retired { return errAssemblyDebianCapture }
	if err := inputs.CheckCurrent(ctx); err != nil { return err }
	return guard.CheckCurrent(ctx)
}

// Retirement never waits for trust eligibility, catalog locks or receipt IO.
// Failed Close retains the exact original owner/guard and remains retire-only.
func (r *assemblyDebianOwnedRun) retire() error {
	if r == nil { return errAssemblyDebianCapture }
	r.mu.Lock()
	if r.phase == "closed" { r.mu.Unlock(); return nil }
	if r.phase == "candidate" { r.mu.Unlock(); return errAssemblyDebianCapture }
	r.phase = "retiring"
	g, inputs := r.guard, r.inputGuard
	r.mu.Unlock()
	if g == nil { return errAssemblyDebianCapture }
	first := g.Close()
	if inputs == nil { return errAssemblyDebianCapture }
	if err := inputs.Close(); first == nil { first = err }
	if first != nil { return first }
	r.mu.Lock()
	r.phase = "closed"
	r.mu.Unlock()
	return nil
}

// Unlike canonical catalog records, producer JSON is pretty printed. Reject
// duplicate keys at every depth before closed DTO decoding, plus trailing data,
// unknown DTO fields, excessive depth/tokens and non-JSON numeric values.
func decodeAssemblyDebianRaw(data []byte, target any) error {
	if len(data) == 0 || len(data) > 256<<10 { return errAssemblyDebianCapture }
	d := json.NewDecoder(bytes.NewReader(data)); d.UseNumber()
	tokens := 0
	var value func(int) error
	value = func(depth int) error {
		tokens++
		if depth > 16 || tokens > 16384 { return errAssemblyDebianCapture }
		t, err := d.Token(); if err != nil { return errAssemblyDebianCapture }
		if delim, ok := t.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token(); if e != nil { return errAssemblyDebianCapture }
					s, ok := k.(string); if !ok || seen[s] { return errAssemblyDebianCapture }; seen[s] = true
					if e = value(depth+1); e != nil { return e }
				}
				end, e := d.Token(); if e != nil || end != json.Delim('}') { return errAssemblyDebianCapture }
			case '[':
				for d.More() { if e := value(depth+1); e != nil { return e } }
				end, e := d.Token(); if e != nil || end != json.Delim(']') { return errAssemblyDebianCapture }
			default: return errAssemblyDebianCapture
			}
		}
		return nil
	}
	if value(0) != nil { return errAssemblyDebianCapture }
	if _, err := d.Token(); err != io.EOF { return errAssemblyDebianCapture }
	d = json.NewDecoder(bytes.NewReader(data)); d.DisallowUnknownFields()
	if d.Decode(target) != nil { return errAssemblyDebianCapture }
	return nil
}

type assemblyDebianRawKey struct {
	Kind string `json:"kind"`
	Created int64 `json:"created"`
	Expires int64 `json:"expires"`
	Algorithm string `json:"algorithm"`
	Bits string `json:"bits"`
	Capabilities string `json:"capabilities"`
	Validity string `json:"validity"`
	Fingerprint string `json:"fingerprint"`
	Primary string `json:"primary"`
}
type assemblyDebianRawKeyObservation struct {
	State string `json:"state"`
	ObservedUTC int64 `json:"observedUtc"`
	Records map[string]assemblyDebianRawKey `json:"records"`
	UnseenRevocations string `json:"unseenRevocations"`
}
type assemblyDebianRawDisposition struct {
	State string `json:"state"`
	Signer string `json:"signer,omitempty"`
	Primary string `json:"primary,omitempty"`
	Created *int64 `json:"created,omitempty"`
	Expires *int64 `json:"expires,omitempty"`
	HashAlgorithm string `json:"hashAlgorithm,omitempty"`
	KeyID string `json:"keyID,omitempty"`
	ErrorCode *int `json:"errorCode,omitempty"`
}
type assemblyDebianRawLogical struct {
	CandidateID string `json:"candidateID"`
	ReleaseDate int64 `json:"releaseDate"`
	ValidUntil *int64 `json:"validUntil"`
	SignatureState string `json:"signatureState"`
	Freshness string `json:"freshness"`
	SignedIndexes []assemblyDebianRawIndex `json:"signedIndexes"`
	HistoricalTimestamp *string `json:"historicalTimestamp"`
	Era string `json:"era"`
	ObservedAt int64 `json:"observedAt"`
	HistoricalAnchor *assemblyDebianHistoricalAnchorV1 `json:"historicalAnchor"`
	UpstreamExpiryState string `json:"upstreamExpiryState"`
	HistoricalExpiryState *string `json:"historicalExpiryState"`
	KeyRevocationKnowledge string `json:"keyRevocationKnowledge"`
	ServerEligibility string `json:"serverEligibility"`
}
type assemblyDebianRawIndex struct {
	Path string `json:"path"`
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
	URL string `json:"url"`
	ByHashURL string `json:"byHashUrl"`
	Read bool `json:"read"`
	SignatureVerified bool `json:"releaseSignatureVerified"`
}
type assemblyDebianRawRelease struct {
	InputSHA string `json:"inputSha256"`
	PlaintextSHA string `json:"signedTextSha256"`
	Dispositions []assemblyDebianRawDisposition `json:"dispositions"`
	Logical []assemblyDebianRawLogical `json:"logicalBindings"`
}
type assemblyDebianRawOutput struct {
	Schema string `json:"schema"`
	TrustPolicySHA string `json:"trustPolicySha256"`
	Keys assemblyDebianRawKeyObservation `json:"keyObservation"`
	Releases []assemblyDebianRawRelease `json:"releases"`
	Consumer string `json:"consumer"`
	Missing []string `json:"missing"`
	MonotonicPolicyState string `json:"monotonicPolicyState"`
}
type assemblyDebianHost struct {
	At string `json:"at"`
	Available int64 `json:"availableBytes"`
	CommitAvailable int64 `json:"commitAvailableBytes"`
	CommitUsed int64 `json:"commitUsedBytes"`
}
type assemblyDebianCPU struct {
	Sequence int `json:"sequence"`
	Known bool `json:"known"`
	Busy *float64 `json:"cpuBusyPercent"`
	Age *float64 `json:"sampleAgeSeconds"`
	ValidAge *float64 `json:"lastValidAgeSeconds"`
	Error *string `json:"error"`
}
type assemblyDebianAdmission struct {
	Host assemblyDebianHost `json:"host"`
	CPU assemblyDebianCPU `json:"cpu"`
}
type assemblyDebianNativeCleanup struct {
	Terminal bool `json:"terminal"`
	JobEmpty bool `json:"jobEmpty"`
	ClosedHandles bool `json:"closedHandlesAcknowledged"`
	PIDs []int `json:"finalOwnedPIDs"`
	Active int `json:"activeProcesses"`
	Unknown []string `json:"handleClosureUnknown"`
}
type assemblyDebianNativeChild struct {
	Index int `json:"index"`
	Kind string `json:"kind"`
	Argv []string `json:"argv"`
	Admission assemblyDebianAdmission `json:"admission"`
	PID int `json:"pid"`
	Began float64 `json:"beganUtc"`
	Ended float64 `json:"endedUtc"`
	Exit int `json:"exitCode"`
	ExitConfirmed bool `json:"childExitConfirmed"`
	JobEmpty bool `json:"jobEmpty"`
	HandlesClosed bool `json:"handlesClosed"`
	BrakeReason *string `json:"brakeReason"`
	BrakeSettled bool `json:"brakeSettled"`
	Cleanup assemblyDebianNativeCleanup `json:"cleanup"`
	StdoutSHA string `json:"stdoutSha256"`
	StderrSHA string `json:"stderrSha256"`
	StdoutBytes int64 `json:"stdoutBytes"`
	StderrBytes int64 `json:"stderrBytes"`
	OutputSHA string `json:"outputSha256,omitempty"`
}
type assemblyDebianNativeReceipt struct {
	Version int `json:"version"`
	Phase string `json:"phase"`
	State string `json:"state"`
	SupervisorSHA string `json:"supervisorSha256"`
	ConfigSHA string `json:"configSha256"`
	PolicySHA string `json:"trustPolicySha256"`
	Maximum int `json:"nativeChildMaximum"`
	Network int `json:"networkRequests"`
	KeyImports int `json:"keyImports"`
	OtherCalls int `json:"modelDockerCompilerCalls"`
	Whole int `json:"wholeSeconds"`
	Work int `json:"workSeconds"`
	CleanupSeconds int `json:"cleanupReserveSeconds"`
	Unknown bool `json:"unknownResources"`
	PublicationUnknown bool `json:"publicationUnknown"`
	Children []assemblyDebianNativeChild `json:"children"`
	Admission assemblyDebianAdmission `json:"admission"`
	Keys assemblyDebianRawKeyObservation `json:"keyObservation"`
	TypedSHA string `json:"typedEvidenceSha256"`
	PDH struct {
		Worker bool `json:"workerSettled"`
		Query struct {
			Attempted bool `json:"attempted"`
			Status int `json:"status"`
			Closed bool `json:"closed"`
			Null bool `json:"queryAndCounterHandlesNull"`
		} `json:"queryCleanup"`
		Error *string `json:"error"`
	} `json:"pdhCleanup"`
	FailedInputCloses int `json:"inputHandleClosureFailures"`
	Elapsed float64 `json:"elapsedSeconds"`
	Within bool `json:"withinWholeDeadline"`
}

// Original outer ownership cannot be established by these JSON fields. Its
// exact raw receipt/final projection hashes instead come from the private live
// completion above. The raw DTO is nevertheless closed and checked for a truthful
// finite projection; it is never treated as a capability or execution authority.
type assemblyDebianOuterFinal struct {
	State string `json:"state"`
	Receipt string `json:"receipt"`
	WriteSettled bool `json:"receiptWriteSettled"`
	SHA string `json:"receiptSHA256"`
	Unknown bool `json:"unknownResources"`
	PublicationUnknown bool `json:"publicationUnknown"`
	Elapsed float64 `json:"finalElapsedSeconds"`
	Within bool `json:"withinWholeDeadline"`
}
type assemblyDebianOuterAdmission struct {
	Observed string `json:"observedUtc"`
	Available int64 `json:"availableBytes"`
	Commit int64 `json:"commitAvailableBytes"`
	Age float64 `json:"sampleAgeSeconds"`
	Sequence int `json:"sequence"`
	Source string `json:"source"`
	CPU float64 `json:"cpuBusyPercent"`
}
type assemblyDebianOuterChild struct {
	Index int `json:"index"`
	Command []string `json:"command"`
	Admission assemblyDebianOuterAdmission `json:"admission"`
	PID int `json:"pid"`
	Start string `json:"startUtc"`
	ExitConfirmed bool `json:"childExitConfirmed"`
	Killed bool `json:"killRequested"`
	Brake *string `json:"brake"`
	Capture bool `json:"captureSettled"`
	Watchdog bool `json:"deadlineWatchdogSettled"`
	WatchdogError *string `json:"deadlineWatchdogError"`
	RetirementError *string `json:"retirementError"`
	Exit int `json:"exitCode"`
	Duration float64 `json:"durationSeconds"`
	Stdout string `json:"stdoutSHA256"`
	Stderr string `json:"stderrSHA256"`
}
type assemblyDebianObserverReceipt struct {
	Command []string `json:"command"`
	PID int `json:"pid"`
	Start string `json:"startUtc"`
	Launched bool `json:"launchAttempted"`
	Original bool `json:"originalHandleOwned"`
	ObserverOnly bool `json:"observerOnly"`
	MemorySource string `json:"memorySource"`
	ImportsLimit int `json:"telemetryHelperImportLimit"`
	Imports int `json:"telemetryHelperImports"`
	Exit bool `json:"childExitConfirmed"`
	Watchdog bool `json:"watchdogSettled"`
	Capture bool `json:"captureSettled"`
	Unknown bool `json:"unknownResources"`
	PublicationUnknown bool `json:"publicationUnknown"`
	WatchdogError *string `json:"watchdogError"`
	DeadlineForced bool `json:"deadlineForced"`
	Samples int `json:"samples"`
	Stdout string `json:"stdoutSHA256"`
	Stderr string `json:"stderrSHA256"`
}
type assemblyDebianOuterReceipt struct {
	Version int `json:"version"`
	Phase string `json:"phase"`
	SupervisorSHA string `json:"supervisorSHA256"`
	DonorSHA string `json:"donorSHA256"`
	Pins []struct{ Path string `json:"path"`; SHA string `json:"sha256"`; Bytes *int64 `json:"bytes,omitempty"` } `json:"sourcePins"`
	PythonSHA string `json:"pythonSHA256"`
	Whole int `json:"wholeSeconds"`
	Work int `json:"workSeconds"`
	CleanupSeconds int `json:"cleanupReserveSeconds"`
	State string `json:"state"`
	Errors []string `json:"errors"`
	Children []assemblyDebianOuterChild `json:"children"`
	Unknown bool `json:"unknownResources"`
	PublicationUnknown bool `json:"publicationUnknown"`
	NativeMain int `json:"nativeHelperMainCalls"`
	TestImports int `json:"testNativeHelperImports"`
	TelemetryLimit int `json:"telemetryHelperImportLimit"`
	TelemetryImports int `json:"telemetryHelperImports"`
	Docker int `json:"dockerCalls"`
	API int `json:"APIStarts"`
	Auth int `json:"authSessions"`
	Models int `json:"modelRequests"`
	Credentials int `json:"credentialReads"`
	ProductWrites int `json:"productStateWrites"`
	InnerSHA string `json:"innerReceiptSHA256"`
	Observer assemblyDebianObserverReceipt `json:"memoryObserver"`
	Elapsed float64 `json:"elapsedSeconds"`
	Within bool `json:"prePublicationWithinWholeDeadline"`
	ElapsedMeasurement string `json:"elapsedMeasurement"`
}

func captureDebianSignatureRun(ctx context.Context, run *assemblyDebianOwnedRun) (*assemblyDebianCapturedRun, error) {
	if run == nil { return nil, errAssemblyDebianCapture }
	run.mu.Lock()
	c := run.completion
	if run.phase != "settled" || c == nil || c.runID != run.id || !checkLogicalID(run.id) || !assemblyCatalogHex(c.outerSHA) || c.exitCode != 0 || !c.originalHandleOwned || !c.exitConfirmed || !c.capturesSettled || !c.watchdogSettled || !c.observerSettled || !c.publicationSettled || !c.handlesSettled || c.unknown || !finiteAssemblyDebianEpoch(c.started) || !finiteAssemblyDebianEpoch(c.completed) || c.completed < c.started || c.completed-c.started > 60 {
		run.mu.Unlock(); return nil, errAssemblyDebianCapture
	}
	run.phase = "capturing"
	guard, inputs, objects := run.guard, run.inputGuard, run.objects
	run.mu.Unlock()
	if run.current(ctx) != nil || guard == nil || !validAssemblyDebianSlots(objects) { return nil, errAssemblyDebianCapture }
	raw := map[string][]byte{}
	total := 0
	for slot, ref := range objects {
		if ref.Bytes < 0 || ref.Bytes > 1<<20 || !assemblyCatalogHex(ref.SHA256) { return nil, errAssemblyDebianCapture }
		reader := guard; if assemblyDebianInputSlot(slot) { reader = inputs }
		// ReadBounded takes a positive ceiling even for an actual empty log.
		maximum := ref.Bytes; if maximum == 0 { maximum = 1 }
		data, err := reader.ReadBounded(ctx, slot, maximum)
		if err != nil || int64(len(data)) != ref.Bytes || assemblyCatalogDigest(data) != ref.SHA256 { return nil, errAssemblyDebianCapture }
		total += len(data); if total > 4<<20 { return nil, errAssemblyDebianCapture }
		raw[slot] = append([]byte(nil), data...)
	}
	if assemblyCatalogDigest(raw["inner-receipt"]) != c.innerReceiptSHA || assemblyCatalogDigest(raw["outer-receipt"]) != c.outerReceiptSHA || assemblyCatalogDigest(raw["outer-final"]) != c.finalProjectionSHA || assemblyCatalogDigest(raw["producer-source-receipt"]) != assemblyDebianSourceReceiptSHA || assemblyCatalogDigest(raw["producer-config"]) != assemblyDebianConfigSHA || assemblyCatalogDigest(raw["inner-source"]) != assemblyDebianInnerSHA || assemblyCatalogDigest(raw["projection-source"]) != assemblyDebianProjectionSHA || assemblyCatalogDigest(raw["outer-source"]) != c.outerSHA || assemblyCatalogDigest(raw["dependency-lock"]) != assemblyDebianLockSHA || assemblyCatalogDigest(raw["acquisition-manifest"]) != assemblyDebianManifestSHA || assemblyCatalogDigest(raw["snapshot-source"]) != "615c3e2f894afdad54d0ff676117ae6be513b4bf500b05660cecbcb5238f7feb" { return nil, errAssemblyDebianCapture }
	var final assemblyDebianOuterFinal
	var outer assemblyDebianOuterReceipt
	var inner assemblyDebianNativeReceipt
	var output assemblyDebianRawOutput
	if decodeAssemblyDebianRaw(raw["outer-final"], &final) != nil || final.State != "retained-signature-batch-positive-cleanup" || final.SHA != c.outerReceiptSHA || !final.WriteSettled || final.Unknown || final.PublicationUnknown || !final.Within || final.Elapsed < 0 || final.Elapsed > 60 || decodeAssemblyDebianRaw(raw["inner-receipt"], &inner) != nil || decodeAssemblyDebianRaw(raw["producer-output"], &output) != nil { return nil, errAssemblyDebianCapture }
	if decodeAssemblyDebianRaw(raw["outer-receipt"], &outer)!=nil || outer.Version!=1 || outer.Phase!="retained-only-debian-signature" || outer.SupervisorSHA!=c.outerSHA || outer.InnerSHA!=c.innerReceiptSHA || outer.State!=final.State || outer.Unknown || outer.PublicationUnknown || !outer.Within || outer.Elapsed<0 || outer.Elapsed>final.Elapsed || outer.Whole!=60 || outer.Work!=40 || outer.CleanupSeconds!=20 || len(outer.Errors)!=0 || len(outer.Children)!=1 || outer.NativeMain!=0 || outer.TestImports!=0 || outer.TelemetryLimit!=1 || outer.TelemetryImports!=1 || outer.Docker!=0 || outer.API!=0 || outer.Auth!=0 || outer.Models!=0 || outer.Credentials!=0 || outer.ProductWrites!=0 { return nil,errAssemblyDebianCapture }
	oc,observer:=outer.Children[0],outer.Observer
	if oc.Index!=0 || len(c.innerCommand)==0 || !equalAssemblyDebianJSON(oc.Command,c.innerCommand) || oc.PID<=0 || !oc.ExitConfirmed || oc.Killed || oc.Brake!=nil || !oc.Capture || !oc.Watchdog || oc.WatchdogError!=nil || oc.RetirementError!=nil || oc.Exit!=0 || !observer.Launched || !observer.Original || !observer.ObserverOnly || observer.PID<=0 || !observer.Exit || !observer.Watchdog || !observer.Capture || observer.Unknown || observer.PublicationUnknown || observer.WatchdogError!=nil || observer.Imports!=1 || observer.ImportsLimit!=1 || observer.Samples<=0 || observer.MemorySource!="frozen GetPerformanceInfo" || assemblyCatalogDigest(raw["outer-stdout"])!=oc.Stdout || assemblyCatalogDigest(raw["outer-stderr"])!=oc.Stderr || assemblyCatalogDigest(raw["observer-stdout"])!=observer.Stdout || assemblyCatalogDigest(raw["observer-stderr"])!=observer.Stderr { return nil,errAssemblyDebianCapture }
	if inner.Version != 1 || inner.Phase != "retained-release-signature-verification" || inner.State != "retained-signature-evidence-complete-with-explicit-gaps" || inner.SupervisorSHA != assemblyDebianInnerSHA || inner.ConfigSHA != assemblyDebianConfigSHA || inner.PolicySHA != assemblyDebianPolicySourceSHA || inner.Maximum != 9 || inner.Network != 0 || inner.KeyImports != 0 || inner.OtherCalls != 0 || inner.Whole != 60 || inner.Work != 40 || inner.CleanupSeconds != 20 || inner.Unknown || inner.PublicationUnknown || !inner.Within || inner.Elapsed < 0 || inner.Elapsed > 60 || inner.FailedInputCloses != 0 || !inner.PDH.Worker || !inner.PDH.Query.Attempted || inner.PDH.Query.Status!=0 || !inner.PDH.Query.Closed || !inner.PDH.Query.Null || inner.PDH.Error != nil || len(inner.Children) != 9 || inner.TypedSHA != assemblyCatalogDigest(raw["producer-output"]) { return nil, errAssemblyDebianCapture }
	if output.Schema != "luci-retained-debian-signature-input-evidence-v2" || output.TrustPolicySHA != assemblyDebianPolicySourceSHA || len(output.Releases) != 5 || output.MonotonicPolicyState != "not-updated-by-this-read-only-batch" || output.Consumer != "server-captured AssemblyInputClosureRef; mandatory ValidateCurrent" || !equalAssemblyDebianJSON(inner.Keys, output.Keys) { return nil, errAssemblyDebianCapture }
	missing := []string{"index-paragraphs", "package-source-bindings", "installed-manifest", "source-retention", "notice-closure", "output-proof"}
	if !equalAssemblyDebianJSON(output.Missing, missing) { return nil, errAssemblyDebianCapture }
	logBytes:=0
	for i, child := range inner.Children {
		kind := "verify"; if i < 3 { kind = "dearmor" } else if i == 3 { kind = "keys" }
		if child.Index != i || child.Kind != kind || len(run.commands[i]) == 0 || !equalAssemblyDebianJSON(child.Argv, run.commands[i]) || child.PID <= 0 || !finiteAssemblyDebianEpoch(child.Began) || !finiteAssemblyDebianEpoch(child.Ended) || child.Began < c.started || child.Ended < child.Began || child.Ended > c.completed || i > 0 && child.Began < inner.Children[i-1].Ended || !child.ExitConfirmed || !child.JobEmpty || !child.HandlesClosed || !child.BrakeSettled || child.BrakeReason != nil || !child.Cleanup.Terminal || !child.Cleanup.JobEmpty || !child.Cleanup.ClosedHandles || child.Cleanup.Active != 0 || len(child.Cleanup.PIDs) != 0 || len(child.Cleanup.Unknown) != 0 || i < 4 && child.Exit != 0 || i >= 4 && child.Exit != 0 && child.Exit != 2 { return nil, errAssemblyDebianCapture }
		if int64(len(raw["stdout-"+strconv.Itoa(i)])) != child.StdoutBytes || int64(len(raw["stderr-"+strconv.Itoa(i)])) != child.StderrBytes || assemblyCatalogDigest(raw["stdout-"+strconv.Itoa(i)]) != child.StdoutSHA || assemblyCatalogDigest(raw["stderr-"+strconv.Itoa(i)]) != child.StderrSHA { return nil, errAssemblyDebianCapture }
		logBytes+=int(child.StdoutBytes+child.StderrBytes)
		if child.Admission.Host.Available<8<<30 || child.Admission.Host.CommitAvailable<8<<30 || !child.Admission.CPU.Known || child.Admission.CPU.Busy==nil || *child.Admission.CPU.Busy<0 || *child.Admission.CPU.Busy>=50 || child.Admission.CPU.Error!=nil { return nil,errAssemblyDebianCapture }
		if i < 3 && child.OutputSHA != assemblyCatalogDigest(raw["keyring-"+strconv.Itoa(i)]) || i >= 4 && child.OutputSHA != assemblyCatalogDigest(raw["plaintext-"+strconv.Itoa(i-4)]) { return nil, errAssemblyDebianCapture }
	}
	if logBytes>256<<10 || len(raw["outer-stdout"])+len(raw["outer-stderr"])>256<<10 || len(raw["observer-stdout"])>65536 || len(raw["observer-stderr"])>65536 { return nil,errAssemblyDebianCapture }
	if run.current(ctx) != nil { return nil, errAssemblyDebianCapture }
	run.mu.Lock()
	defer run.mu.Unlock()
	if run.phase != "capturing" { return nil, errAssemblyDebianCapture }
	run.phase = "captured"
	return &assemblyDebianCapturedRun{run, raw, inner, output}, nil
}

func equalAssemblyDebianJSON(a, b any) bool { x, _ := json.Marshal(a); y, _ := json.Marshal(b); return bytes.Equal(x, y) }
func finiteAssemblyDebianEpoch(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 && v <= 4102444800 }
func optionalAssemblyDebianEpoch(v int64) *int64 { if v == 0 { return nil }; return &v }

type assemblyDebianDomainCandidate struct {
	rawManifestRef assemblyCatalogRef
	mu sync.Mutex
	indexTransfer bool // latch/serialize handoff; never held across native IO
	indexHandoff *assemblyDebianIndexCandidate // exact promoted retirement owner
	guard, inputs backend.DocumentImageAssemblyEvidenceGuard
	retiring bool
	adopting bool
	targetCAS assemblyCatalogCAS
	targetIndex assemblyCatalogIndex
	publication *assemblyDebianPublicationPermit
	publicationUnknown, publicationRetired bool
	capture *assemblyDebianCapturedRun
	policy assemblyDebianRootPolicyV1
	keys assemblyDebianKeyObservationsV1
	signatures assemblyDebianReleaseSignaturesV1
	pins assemblyDebianProducerPins
	records []assemblyCatalogObject
}

func newAssemblyDebianDomains(ctx context.Context, owner *assemblyInputOwner, captured *assemblyDebianCapturedRun, now int64) (*assemblyDebianDomainCandidate, error) {
	if captured == nil || captured.run == nil || captured.run.owner != owner || captured.run.current(ctx) != nil { return nil, errAssemblyDebianCapture }
	r := captured.run
	if r.product!=nil||r.completion!=nil&&r.completion.product!=nil{return nil,errAssemblyDebianCapture}
	policy, err := normalizeAssemblyDebianPolicy(captured.raw["policy-source"], captured.raw["producer-config"], r.policy.Revision)
	if err != nil { return nil, err }
	ref := func(slot, role string) assemblyDebianObjectRef { data := captured.raw[slot]; return assemblyDebianObjectRef{role, assemblyCatalogDigest(data), int64(len(data))} }
	pins := assemblyDebianProducerPins{SourceReceipt: ref("producer-source-receipt", "producer-source-receipt"), Config: policy.ProducerConfig, ProjectionSHA256: assemblyDebianProjectionSHA, GPGSHA256: "a5140c85353e8399da8d6bd7e3741524cd76a4e69be88af12042b1c9ef022984", GPGVSHA256: "f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b"}
	producer := assemblyDebianProducerV1{Profile: captured.output.Schema, SourceReceipt: pins.SourceReceipt, Config: pins.Config, TypedOutput: ref("producer-output", "producer-output"), InnerReceipt: ref("inner-receipt", "inner-receipt"), OuterReceipt: ref("outer-receipt", "outer-receipt"), ProjectionSHA256: pins.ProjectionSHA256, GPGSHA256: pins.GPGSHA256, GPGVSHA256: pins.GPGVSHA256}
	return newAssemblyDebianMappedDomains(ctx,owner,captured,now,policy,pins,producer)
}
func newAssemblyDebianProductDomains(ctx context.Context,owner *assemblyInputOwner,captured *assemblyDebianCapturedRun,now int64)(*assemblyDebianDomainCandidate,error){
	if captured==nil||captured.run==nil||captured.run.owner!=owner||captured.run.product==nil||captured.run.completion==nil||captured.run.completion.product==nil||captured.run.current(ctx)!=nil||captured.output.Schema!=assemblyDebianProductRawSchema{return nil,errAssemblyDebianCapture}
	r:=captured.run;completion:=r.completion.product;selected:=completion.pins
	if !validAssemblyDebianProductPins(selected)||completion.profileSHA!=selected.Profile.SHA256||completion.identity!=r.product.identity||!completion.native.positiveCompletion(completion.children){return nil,errAssemblyDebianCapture}
	policy,e:=normalizeAssemblyDebianProductPolicy(captured.raw["policy-source"],captured.raw["producer-config"],selected,r.policy.Revision);if e!=nil{return nil,e}
	ref:=func(slot,role string)assemblyDebianObjectRef{data:=captured.raw[slot];return assemblyDebianObjectRef{role,assemblyCatalogDigest(data),int64(len(data))}}
	pins:=assemblyDebianProducerPins{Product:&selected,SourceReceipt:selected.SourceReceipt,Config:selected.Config,ProjectionSHA256:selected.Projection.SHA256,GPGSHA256:selected.GPG.SHA256,GPGVSHA256:selected.GPGV.SHA256}
	if ref("producer-source-receipt","producer-source-receipt")!=selected.SourceReceipt||ref("projection-source","projection-source")!=selected.Projection{return nil,errAssemblyDebianCapture}
	producer:=assemblyDebianProducerV1{ProductOwner:&assemblyDebianProductOwnerV1{Protocol:assemblyDebianProductProtocol,Profile:selected.Profile,Receipt:ref("native-owner-receipt","product-native-owner-receipt"),Entry:selected.Entry,Python:selected.Python},Profile:assemblyDebianProductRawSchema,SourceReceipt:selected.SourceReceipt,Config:selected.Config,TypedOutput:ref("producer-output","producer-output"),ProjectionSHA256:selected.Projection.SHA256,GPGSHA256:selected.GPG.SHA256,GPGVSHA256:selected.GPGV.SHA256}
	if producer.ProductOwner.Receipt.SHA256!=completion.receiptSHA||checkAssemblyDebianProducer(producer,pins)!=nil{return nil,errAssemblyDebianCapture}
	return newAssemblyDebianMappedDomains(ctx,owner,captured,now,policy,pins,producer)
}
func newAssemblyDebianMappedDomains(ctx context.Context,owner *assemblyInputOwner,captured *assemblyDebianCapturedRun,now int64,policy assemblyDebianRootPolicyV1,pins assemblyDebianProducerPins,producer assemblyDebianProducerV1)(*assemblyDebianDomainCandidate,error){
	r:=captured.run
	ref:=func(slot,role string)assemblyDebianObjectRef{data:=captured.raw[slot];return assemblyDebianObjectRef{role,assemblyCatalogDigest(data),int64(len(data))}}
	policyRef, _ := assemblyDebianRecordRef("official-root-policy", policy)
	keys := assemblyDebianKeyObservationsV1{Schema: 1, RecordType: "luci-debian-key-observations-v1", RootPolicy: policyRef, Producer: producer, ObservedAt: captured.output.Keys.ObservedUTC, KeyListing: ref("stdout-3", "key-listing"), UnseenRevocations: captured.output.Keys.UnseenRevocations}
	if captured.output.Keys.State != "pinned-official-key-snapshot-identities-derived" { return nil, errAssemblyDebianCapture }
	for i, root := range policy.Roots {
		if ref("public-key-"+strconv.Itoa(i), "public-key") != root.PublicKey { return nil, errAssemblyDebianCapture }
		keys.Snapshots = append(keys.Snapshots, assemblyDebianKeySnapshotV1{root.ID, root.PublicKey, ref("keyring-"+strconv.Itoa(i), "keyring")})
	}
	for fingerprint, key := range captured.output.Keys.Records {
		algorithm, aerr := strconv.Atoi(key.Algorithm); bits, berr := strconv.Atoi(key.Bits)
		if aerr != nil || berr != nil || key.Fingerprint != fingerprint { return nil, errAssemblyDebianCapture }
		keys.Keys = append(keys.Keys, assemblyDebianKeyV1{fingerprint, key.Primary, key.Kind, key.Created, optionalAssemblyDebianEpoch(key.Expires), algorithm, bits, key.Capabilities, key.Validity})
	}
	sort.Slice(keys.Keys, func(i,j int) bool { return keys.Keys[i].Fingerprint < keys.Keys[j].Fingerprint })
	keyBytes, _ := json.Marshal(keys)
	keys, err := decodeAssemblyDebianKeySummary(keyBytes, policy, pins, now); if err != nil { return nil, err }
	keyRef, _ := assemblyDebianRecordRef("key-observations", keys)
	signatures := assemblyDebianReleaseSignaturesV1{Schema: 1, RecordType: "luci-debian-release-signatures-v1", RootPolicy: policyRef, KeyObservations: keyRef, Producer: producer}
	for i, release := range captured.output.Releases {
		input := ref("signed-release-"+strconv.Itoa(i), "signed-release")
		plain := ref("plaintext-"+strconv.Itoa(i), "signed-plaintext")
		if release.InputSHA != input.SHA256 || release.PlaintextSHA != plain.SHA256 { return nil, errAssemblyDebianCapture }
		pins.SignedObjects = append(pins.SignedObjects, input)
		child := captured.inner.Children[4+i]
		value := assemblyDebianReleaseVerificationV1{SignedRelease: input, SignedPlaintext: plain, MachineStatus: ref("stdout-"+strconv.Itoa(4+i), "signature-status"), ChildIndex: 4+i, ExitCode: child.Exit, ObservedStart: int64(math.Floor(child.Began)), ObservedEnd: int64(math.Ceil(r.completion.completed))}
		for _, d := range release.Dispositions {
			mapped := assemblyDebianSignatureDispositionV1{Kind: d.State}
			switch d.State {
			case "eligible-valid-signature":
				hash, e := strconv.Atoi(d.HashAlgorithm)
				if e != nil || d.Created == nil || d.Expires == nil || d.KeyID != "" || d.ErrorCode != nil { return nil, errAssemblyDebianCapture }
				mapped.Valid = &assemblyDebianValidSignatureV1{d.Signer, d.Primary, *d.Created, optionalAssemblyDebianEpoch(*d.Expires), hash}
			case "unverified-extra-signature":
				if d.ErrorCode == nil || d.Signer != "" || d.Primary != "" || d.Created != nil || d.Expires != nil || d.HashAlgorithm != "" { return nil, errAssemblyDebianCapture }
				mapped.UnverifiedExtra = &assemblyDebianUnverifiedExtraV1{d.KeyID, *d.ErrorCode}
			default: return nil, errAssemblyDebianCapture
			}
			value.Dispositions = append(value.Dispositions, mapped)
		}
		for _, l := range release.Logical {
			indexes := []assemblyDebianSignedIndexClaimV1{}
			for _, index := range l.SignedIndexes {
				if index.Read || index.SignatureVerified { return nil, errAssemblyDebianCapture }
				indexes = append(indexes, assemblyDebianSignedIndexClaimV1{index.Path, index.Bytes, index.SHA256})
			}
			value.Logical = append(value.Logical, assemblyDebianLogicalReleaseV1{CandidateID: l.CandidateID, ReleaseDate: l.ReleaseDate, ValidUntil: l.ValidUntil, ProducerFreshness: l.Freshness, ProducerHistoricalTimestamp: l.HistoricalTimestamp, Era: l.Era, ObservedAt: l.ObservedAt, SignatureState: l.SignatureState, HistoricalAnchor: l.HistoricalAnchor, UpstreamExpiryState: l.UpstreamExpiryState, HistoricalExpiryState: l.HistoricalExpiryState, KeyRevocationKnowledge: l.KeyRevocationKnowledge, ServerEligibility: l.ServerEligibility, SignedIndexes: indexes})
		}
		signatures.Releases = append(signatures.Releases, value)
	}
	signatureBytes, _ := json.Marshal(signatures)
	signatures, err = decodeAssemblyDebianSignatureSummary(signatureBytes, policy, keys, pins, now); if err != nil { return nil, err }
	candidate := &assemblyDebianDomainCandidate{capture: captured, policy: policy, keys: keys, signatures: signatures, pins: pins}
	for _, item := range []struct { kind string; value any }{{"official-root-policy", policy}, {"key-observations", keys}, {"release-signatures", signatures}} {
		data, e := json.Marshal(item.value); if e != nil { return nil, e }
		ref, e := assemblyDebianRecordRef(item.kind, item.value); if e != nil { return nil, e }
		candidate.records = append(candidate.records, assemblyCatalogObject{ref, data})
	}
	if r.current(ctx) != nil { return nil, errAssemblyDebianCapture }
	if pins.Product!=nil{return finishAssemblyDebianProductCandidate(ctx,r,candidate)}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.phase != "captured" { return nil, errAssemblyDebianCapture }
	// Transfer invalidates each old token; partial transfer remains owned by r.
	outputGuard, e := r.guard.Transfer(ctx); if e != nil { return nil, e }; r.guard = outputGuard
	inputGuard, e := r.inputGuard.Transfer(ctx); if e != nil { return nil, e }; r.inputGuard = inputGuard
	candidate.guard, candidate.inputs = outputGuard, inputGuard
	r.guard, r.inputGuard = nil, nil
	r.phase = "candidate"
	return candidate, nil
}

// The new product arm never borrows an owner mutex across original guard IO.
// Late/partial returned tokens are reattached before refusal, so retirement can
// latch independently and retains exact ownership rather than losing a move.
func finishAssemblyDebianProductCandidate(ctx context.Context,r *assemblyDebianOwnedRun,c *assemblyDebianDomainCandidate)(*assemblyDebianDomainCandidate,error){
	r.mu.Lock();if r.phase!="captured"||r.product==nil{r.mu.Unlock();return nil,errAssemblyDebianCapture};r.phase="product-candidate-transfer";output,inputs:=r.guard,r.inputGuard;r.mu.Unlock()
	refuse:=func(e error)(*assemblyDebianDomainCandidate,error){_ = r.retire();return nil,e}
	outputNext,e:=output.Transfer(ctx);r.mu.Lock();if outputNext!=nil{r.guard=outputNext};phase:=r.phase;r.mu.Unlock();if e!=nil{return refuse(e)};if outputNext==nil||phase!="product-candidate-transfer"||ctx.Err()!=nil{return refuse(errAssemblyDebianCapture)}
	inputNext,e:=inputs.Transfer(ctx);r.mu.Lock();if inputNext!=nil{r.inputGuard=inputNext};phase=r.phase;r.mu.Unlock();if e!=nil{return refuse(e)};if inputNext==nil||phase!="product-candidate-transfer"||ctx.Err()!=nil{return refuse(errAssemblyDebianCapture)}
	if e=outputNext.CheckCurrent(ctx);e!=nil{return refuse(e)};if e=inputNext.CheckCurrent(ctx);e!=nil{return refuse(e)};if e=r.product.current(ctx);e!=nil{return refuse(e)}
	r.mu.Lock();if r.phase!="product-candidate-transfer"||r.guard!=outputNext||r.inputGuard!=inputNext||ctx.Err()!=nil{r.mu.Unlock();return refuse(errAssemblyDebianCapture)};c.guard,c.inputs=outputNext,inputNext;r.guard,r.inputGuard=nil,nil;r.phase="candidate";r.mu.Unlock();return c,nil
}

func assemblyDebianInputSlot(slot string) bool {
	switch slot {
	case "policy-source", "producer-source-receipt", "producer-config", "inner-source", "projection-source", "outer-source", "dependency-lock", "acquisition-manifest", "snapshot-source": return true
	}
	for i:=0; i<5; i++ { if slot == "signed-release-"+strconv.Itoa(i) || i<3 && slot == "public-key-"+strconv.Itoa(i) { return true } }
	return false
}

func validAssemblyDebianSlots(objects map[string]assemblyDebianObjectRef) bool {
	expected := map[string]bool{}
	for _, slot := range []string{"policy-source", "producer-source-receipt", "producer-config", "inner-source", "projection-source", "outer-source", "dependency-lock", "acquisition-manifest", "snapshot-source", "inner-receipt", "outer-receipt", "outer-final", "producer-output", "outer-stdout", "outer-stderr", "observer-stdout", "observer-stderr"} { expected[slot] = true }
	for i:=0;i<9;i++ {
		expected["stdout-"+strconv.Itoa(i)], expected["stderr-"+strconv.Itoa(i)] = true,true
		if i<5 { expected["signed-release-"+strconv.Itoa(i)], expected["plaintext-"+strconv.Itoa(i)] = true,true }
		if i<3 { expected["public-key-"+strconv.Itoa(i)], expected["keyring-"+strconv.Itoa(i)] = true,true }
	}
	if len(objects) != len(expected) { return false }
	for slot := range objects { if !expected[slot] { return false } }
	return true
}
