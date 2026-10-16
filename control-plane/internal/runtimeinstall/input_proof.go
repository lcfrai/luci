package runtimeinstall

import (
 "context"
 "errors"
 "strings"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/management"
)

const DebianInputProofKind = "document-input-proof"
const DebianInputProofProtocol = "luci-debian-signature-go-owner-v1"
var ErrInputProofUnavailable = errors.New("the application-pinned Debian proof profile and licensed tool closure are unavailable")
var ErrInputProofOwnership = errors.New("the current installer no longer owns this input proof")

type InputProofCAS struct { Revision uint64 `json:"revision"`; Digest string `json:"digest"` }
type DebianInputProofIdentity struct {
 JobID string `json:"jobId"`
 JobGeneration string `json:"jobGeneration"`
 RunID string `json:"runId"`
 AllocationID string `json:"allocationId"`
 ProfileID string `json:"profileId"`
 ProfileSHA256 string `json:"profileSHA256"`
 SourceSHA256 string `json:"sourceSHA256"`
 ExpectedCAS InputProofCAS `json:"expectedCas"`
 OwnerPID uint32 `json:"ownerPid,omitempty"`
 OwnerCreationTime uint64 `json:"ownerCreationTime,omitempty"`
}
type DebianInputProofRequest struct { Identity DebianInputProofIdentity; Actor management.ExecutionActorRef }
type InputProofDomainRef struct { Kind string `json:"kind"`; SHA256 string `json:"sha256"`; Bytes int64 `json:"bytes"` }
type DebianInputProofOutcome struct {
 Identity DebianInputProofIdentity `json:"identity"`
 Phase string `json:"phase"`
 ResourcesHeld bool `json:"resourcesHeld"`
 CleanupConfirmed bool `json:"cleanupConfirmed"`
 PublicationUnknown bool `json:"publicationUnknown"`
 ReasonCode string `json:"reasonCode,omitempty"`
 Message string `json:"message"`
 PublishedCAS InputProofCAS `json:"publishedCas"`
 Domains []InputProofDomainRef `json:"domains,omitempty"`
}
type PublishDebianInputProof func(DebianInputProofOutcome) error

// The application build selects these bytes. A manifest's own digest is not a
// trust root. Browser input cannot supply this descriptor or its root directory.
type DebianInputProofFile struct { Role string `json:"role"`; RelativePath string `json:"relativePath"`; Bytes int64 `json:"bytes"`; SHA256 string `json:"sha256"` }
type BundledDebianInputProofProfile struct {
 ID string `json:"id"`
 Protocol string `json:"protocol"`
 SHA256 string `json:"sha256"`
 SourceSHA256 string `json:"sourceSHA256"`
 Files []DebianInputProofFile `json:"files"`
 // Root is set by the Product owner, never decoded from manifest JSON.
 Root string `json:"-"`
}
func InputProofHex(s string) bool { return len(s)==64 && strings.Trim(s,"0123456789abcdef")=="" }
func inputProofID(s string)bool{return len(s)>0&&len(s)<=128&&strings.Trim(s,"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")==""}
func ValidInputProofIdentity(v DebianInputProofIdentity) bool {
 return inputProofID(v.JobID) && inputProofID(v.JobGeneration) && inputProofID(v.RunID) && inputProofID(v.AllocationID) && inputProofID(v.ProfileID) && InputProofHex(v.ProfileSHA256) && InputProofHex(v.SourceSHA256) && ((v.ExpectedCAS.Revision==0 && v.ExpectedCAS.Digest=="") || (v.ExpectedCAS.Revision>0 && InputProofHex(v.ExpectedCAS.Digest))) && ((v.OwnerPID==0)==(v.OwnerCreationTime==0))
}
func SameInputProofOperation(a,b DebianInputProofIdentity) bool {
 beforePID,beforeTime:=a.OwnerPID,a.OwnerCreationTime
 if a.ExpectedCAS==(InputProofCAS{}) {a.ExpectedCAS=b.ExpectedCAS}
 a.OwnerPID,a.OwnerCreationTime=0,0;bPID,bTime:=b.OwnerPID,b.OwnerCreationTime;b.OwnerPID,b.OwnerCreationTime=0,0
 return a==b && (beforePID==0 || beforePID==bPID && beforeTime==bTime)
}
func ValidInputProofOutcome(v DebianInputProofOutcome) bool {
 if !ValidInputProofIdentity(v.Identity) || len(v.Message)>2000 || len(v.ReasonCode)>128 || len(v.Domains)>4 || v.ResourcesHeld && v.CleanupConfirmed { return false }
 switch v.Phase { case "queued","reserved","preparing","creating-suspended","running","settling","retiring","unknown":
 case "facts-recorded": if v.ResourcesHeld || !v.CleanupConfirmed || v.PublicationUnknown || v.PublishedCAS.Revision==0 || !InputProofHex(v.PublishedCAS.Digest) || len(v.Domains)!=4 { return false }
 case "blocked","failed","cancelled": if v.ResourcesHeld || !v.CleanupConfirmed { return false }
 default:return false }
 seen:=map[string]bool{}
 for _,r:=range v.Domains { if seen[r.Kind] || !InputProofHex(r.SHA256) || r.Bytes<=0 || r.Bytes>64<<10 {return false};switch r.Kind {case "official-root-policy","key-observations","release-signatures","producer-raw-manifest":default:return false};seen[r.Kind]=true }
 return true
}

// These times are server-owned; no clock or native command comes from HTTP.
type InputProofDeadlines struct { Created,Work,Whole time.Time }
type InputProofObserver = func(context.Context, DebianInputProofIdentity, management.ExecutionActorRef, PublishDebianInputProof)(DebianInputProofOutcome,error)
