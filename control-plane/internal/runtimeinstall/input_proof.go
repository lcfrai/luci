package runtimeinstall

import (
 "bytes"
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "strings"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/management"
)

const DebianInputProofKind = "document-input-proof"
const DebianInputProofProtocol = "luci-debian-signature-go-owner-v1"
// Empty purpose preserves the original ten-stage signature proof. A binary
// proof is a separate captured resource contract, never an argv override.
const DebianBinaryInputProofPurpose = "binary-bindings"
type InputProofLimits struct { Children,MemoryMiB,CPUMillis int; DiskBytes uint64; Work,Whole time.Duration }
func DebianInputProofLimits(purpose string)(InputProofLimits,bool){
 switch purpose {
 case "":return InputProofLimits{10,192,1000,70<<20,40*time.Second,60*time.Second},true
 case DebianBinaryInputProofPurpose:return InputProofLimits{1,2048,1000,128<<20,300*time.Second,420*time.Second},true
 default:return InputProofLimits{},false
 }
}
var ErrInputProofUnavailable = errors.New("the application-pinned Debian proof profile and licensed tool closure are unavailable")
var ErrInputProofOwnership = errors.New("the current installer no longer owns this input proof")

type InputProofCAS struct { Revision uint64 `json:"revision"`; Digest string `json:"digest"` }
type DebianInputProofIdentity struct {
 Purpose string `json:"purpose,omitempty"`
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
 Purpose string `json:"purpose,omitempty"`
 ID string `json:"id"`
 Protocol string `json:"protocol"`
 SHA256 string `json:"sha256"`
 SourceSHA256 string `json:"sourceSHA256"`
 Files []DebianInputProofFile `json:"files"`
 // Root is set by the Product owner, never decoded from manifest JSON.
 Root string `json:"-"`
 // Descriptor is set only by the Product-owned primary/stage loader. It is
 // never decoded from the descriptor itself or accepted from HTTP.
 DescriptorRelativePath string `json:"-"`
}

const DebianSignatureProfileRole="signature-profile"
const DebianSignatureProfilePath="profiles/signature-profile.json"

// Installer-owned configuration supplies independent expected hashes. Neither
// HTTP input nor a digest inside the distributed descriptor establishes trust.
type DocumentRuntimeConfiguration struct {
 ProfileSHA256 string
 ToolDomainSHA256 string
}
func(c DocumentRuntimeConfiguration)Configured()bool{return c!=(DocumentRuntimeConfiguration{})}
func(c DocumentRuntimeConfiguration)Valid()bool{return !c.Configured()||InputProofHex(c.ProfileSHA256)&&InputProofHex(c.ToolDomainSHA256)}

type bundledDebianProfileDescriptor struct {
 Schema int `json:"schema"`
 ID string `json:"id"`
 Protocol string `json:"protocol"`
 Source string `json:"sourceSHA256"`
 Files []DebianInputProofFile `json:"files"`
 Purpose string `json:"purpose,omitempty"`
}
func CanonicalDebianInputProofProfile(p BundledDebianInputProofProfile)([]byte,error){
 return json.Marshal(bundledDebianProfileDescriptor{1,p.ID,p.Protocol,p.SourceSHA256,p.Files,p.Purpose})
}
// Exact canonical bytes reject duplicate/unknown keys and preserve the legacy
// descriptor layout. File/type/current-owner checks remain at the caller.
func DecodeDebianInputProofProfile(data []byte,expected string)(BundledDebianInputProofProfile,error){
 var empty BundledDebianInputProofProfile
 if !InputProofHex(expected)||len(data)==0||len(data)>64<<10{return empty,ErrInputProofUnavailable}
 digest:=sha256.Sum256(data);if hex.EncodeToString(digest[:])!=expected{return empty,ErrInputProofUnavailable}
 var d bundledDebianProfileDescriptor
 if json.Unmarshal(data,&d)!=nil||d.Schema!=1{return empty,ErrInputProofUnavailable}
 p:=BundledDebianInputProofProfile{Purpose:d.Purpose,ID:d.ID,Protocol:d.Protocol,SHA256:expected,SourceSHA256:d.Source,Files:d.Files}
 canonical,err:=CanonicalDebianInputProofProfile(p)
 if err!=nil||!bytes.Equal(data,canonical)||!inputProofID(p.ID)||p.Protocol!=DebianInputProofProtocol||!InputProofHex(p.SourceSHA256)||len(p.Files)==0||len(p.Files)>212{return empty,ErrInputProofUnavailable}
 if _,ok:=DebianInputProofLimits(p.Purpose);!ok{return empty,ErrInputProofUnavailable}
 return p,nil
}
func InputProofHex(s string) bool { return len(s)==64 && strings.Trim(s,"0123456789abcdef")=="" }
func inputProofID(s string)bool{return len(s)>0&&len(s)<=128&&strings.Trim(s,"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-")==""}
func ValidInputProofIdentity(v DebianInputProofIdentity) bool {
 if _,ok:=DebianInputProofLimits(v.Purpose);!ok{return false}
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
