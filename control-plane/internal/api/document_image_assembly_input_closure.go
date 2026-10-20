package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// A comparable handle to an immutable server-owned typed input-evidence record.
// Syntax is not closure, authentication, permission, or installed-output proof.
// Only the concrete guarded constructor below can supply accepted contents.
type assemblyInputClosureRef struct {
	Schema int    `json:"schema"`
	Digest string `json:"digest"`
}

func validAssemblyInputClosureRef(ref assemblyInputClosureRef) bool {
	if ref.Schema != 1 || len(ref.Digest) != 64 {
		return false
	}
	for _, c := range ref.Digest {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// Old unbound rows remain readable and retire/fence-only. They never acquire
// new execution authority merely because a later record has the same inputs.
func retainedAssemblyInputClosureRef(ref assemblyInputClosureRef) bool {
	return ref == (assemblyInputClosureRef{}) || validAssemblyInputClosureRef(ref)
}

type assemblyClosurePageV1 struct {
	Schema int `json:"schema"`
	RecordType string `json:"recordType"`
	Kind string `json:"kind"`
	Part int `json:"part"`
	Rows []json.RawMessage `json:"rows"`
}

type assemblyClosureDomainV1 struct {
	Schema int `json:"schema"`
	RecordType string `json:"recordType"`
	Kind string `json:"kind"`
	Pages []assemblyCatalogRef `json:"pages"`
}

type assemblyClosureDomainRef struct {
	Kind string `json:"kind"`
	Root assemblyCatalogRef `json:"root"`
}

type assemblyClosureRootV1 struct {
	Schema int `json:"schema"`
	RecordType string `json:"recordType"`
	ExpectedCAS assemblyCatalogCAS `json:"expectedCas"`
	Source assemblyCatalogSource `json:"source"`
	Policy assemblyCatalogPolicy `json:"policy"`
	ProducerProfileSHA256 string `json:"producerProfileSha256"`
	ProducerSourceSHA256 string `json:"producerSourceSha256"`
	ToolDomainSHA256 string `json:"toolDomainSha256"`
	Manifest backend.DocumentImageAssemblyManifest `json:"manifest"`
	Domains []assemblyClosureDomainRef `json:"domains"`
}

type assemblyClosureBinaryRow struct {
	Root *assemblyDebianBinariesV1 `json:"root,omitempty"`
	TrustPolicy *assemblyDebianRootPolicyV1 `json:"trustPolicy,omitempty"`
	TrustKeys *assemblyDebianKeyObservationsV1 `json:"trustKeys,omitempty"`
	TrustSignatures *assemblyDebianReleaseSignaturesV1 `json:"trustSignatures,omitempty"`
	TrustPins *assemblyDebianProducerPins `json:"trustPins,omitempty"`
	IndexSelection *assemblyDebianIndexSelectionV1 `json:"indexSelection,omitempty"`
	Part *assemblyDebianBinaryPartV1 `json:"part,omitempty"`
	Controls []assemblyDependencyPackage `json:"controls,omitempty"`
	ClaimPart *assemblyClosureClaimPart `json:"claimPart,omitempty"`
}

type assemblyClosureClaimPart struct { Ordinal int `json:"ordinal"`;Part int `json:"part"`;Data string `json:"data"` }

func assemblyProjectionFromClaimBytes(raw []byte,binding assemblyDebianBinaryBindingV1)(assemblyDependencyPackage,error) {
	if len(raw)==0||len(raw)>65536||int64(len(raw))!=binding.SelectedFieldClaims.Bytes||assemblyCatalogDigest(raw)!=binding.SelectedFieldClaims.SHA256||checkAssemblyBinaryClaimKeys(raw)!=nil{return assemblyDependencyPackage{},errAssemblyClosureDomain}
	var claims assemblyDebianBinarySelectedFieldsV1
	if decodeAssemblyDebianRaw(raw,&claims)!=nil||claims.Schema!=1||len(claims.Indexes)!=len(binding.Matches){return assemblyDependencyPackage{},errAssemblyClosureDomain}
	for i,claim:=range claims.Indexes {if !equalAssemblyDebianJSON(claim.BindingOrdinals,binding.Matches[i].IndexBindingOrdinals){return assemblyDependencyPackage{},errAssemblyClosureDomain}}
	value,err:=assemblyDependencyFromControl(claims.Control);if err!=nil||value.Name!=binding.Name||value.Version!=binding.Version||value.Architecture!=binding.Architecture{return assemblyDependencyPackage{},errAssemblyClosureDomain};return value,nil
}

type assemblyClosureDependencyRow struct {
	Package *assemblyDependencyPackage `json:"package,omitempty"`
	Edge *assemblyDependencyEdge `json:"edge,omitempty"`
	LockPart *assemblyClosureLockPart `json:"lockPart,omitempty"`
}

type assemblyClosureLockPart struct { Part int `json:"part"`;Data string `json:"data"` }
type assemblyVerificationFile struct { input backend.DocumentImageAssemblyInput;data []byte }
type assemblyInstalledVerificationV1 struct {
	Schema int `json:"schema"`
	SourceLockSHA256 string `json:"sourceLockSha256"`
	Source assemblyCatalogSource `json:"source"`
	Helper assemblyObservedHelper `json:"helper"`
	Packages []assemblyDependencyPackage `json:"packages"`
	Wheels []assemblyObservedWheel `json:"wheels"`
	InstalledInputs []backend.DocumentImageAssemblyInput `json:"installedInputs"`
	Notices []backend.DocumentImageAssemblyInput `json:"notices"`
}
type assemblyClosedLockV1 struct {
	Schema int `json:"schema"`
	State string `json:"state"`
	Unresolved []string `json:"unresolved"`
	SourceMetadata string `json:"sourceMetadataBase64"`
	Observed assemblyInstalledVerificationV1 `json:"luciObserved"`
}

func deriveAssemblyVerificationFiles(files []assemblyObservedFile)([]assemblyVerificationFile,error) {
	ordered:=append([]assemblyObservedFile(nil),files...);sort.Slice(ordered,func(i,j int)bool{return ordered[i].Path<ordered[j].Path})
	rows,err:=assemblyClosureRows(ordered);if err!=nil{return nil,err}
	out:=[]assemblyVerificationFile{};total:=0
	for at:=0;at<len(rows); {
		end,size:=at,2
		for end<len(rows)&&size+len(rows[end])+1<=1<<20 {size+=len(rows[end])+1;end++}
		if end==at||len(out)>=64{return nil,errAssemblyClosureDomain}
		data,e:=json.Marshal(rows[at:end]);if e!=nil{return nil,e};total+=len(data);if total>32<<20{return nil,errAssemblyClosureDomain}
		input:=backend.DocumentImageAssemblyInput{Path:fmt.Sprintf("verification/installed-%04d.json",len(out)),Bytes:int64(len(data)),Digest:assemblyCatalogDigest(data)}
		out=append(out,assemblyVerificationFile{input,data});at=end
	}
	return out,nil
}

func deriveAssemblyClosedLock(original []byte,source assemblyCatalogSource,dependency assemblyDependencyClosureV1,domains *assemblyObservedDomains,verification []assemblyVerificationFile,notices []backend.DocumentImageAssemblyInput)([]byte,error) {
	if len(original)==0||len(original)>1<<20||assemblyCatalogDigest(original)!=source.DependencyLockDigest||!json.Valid(original)||domains==nil||len(verification)==0{return nil,errAssemblyClosureDomain}
	observed:=assemblyInstalledVerificationV1{Schema:1,SourceLockSHA256:source.DependencyLockDigest,Source:source,Helper:domains.helper,Packages:dependency.Packages,Wheels:domains.wheels,Notices:notices}
	for _,file:=range verification {observed.InstalledInputs=append(observed.InstalledInputs,file.input)}
	// Original metadata remains attributed and nested as historical input. No
	// unresolved source flag inside it is silently rewritten into an observation.
	data,err:=json.Marshal(assemblyClosedLockV1{1,"closed-reviewed",[]string{},base64.StdEncoding.EncodeToString(original),observed})
	if err!=nil||len(data)>2<<20{return nil,errAssemblyClosureDomain};return data,nil
}

// All fields are private. This candidate is never decoded from a descriptor or
// HTTP request. The original plan, binary spool and inspection guards stay live
// until the existing owner completes publication and explicitly retires them.
type assemblyAcceptedInputCandidate struct {
	mu sync.Mutex
	adopting,retiring,publicationUnknown,publicationRetired bool
	targetCAS assemblyCatalogCAS
	targetIndex assemblyCatalogIndex
	publication *assemblyDebianPublicationPermit
	owner *assemblyInputOwner
	plan *debianProducerPlan
	binary *debianBinarySemanticCapture
	observed *assemblyObservedDomains
	expected assemblyCatalogCAS
	source assemblyCatalogSource
	policy assemblyCatalogPolicy
	manifest backend.DocumentImageAssemblyManifest
	records []assemblyCatalogObject
	closure assemblyInputClosureRef
	closedLock []byte
	closedLockInput backend.DocumentImageAssemblyInput
	verification []assemblyVerificationFile
	profileSHA256,profileSourceSHA256,toolDomainSHA256 string
}

// Resolved metadata is distinct from the live publication candidate. Its
// current check retains the exact selected head, source, policy and record set;
// it never recreates a native producer from a historical JSON receipt.
type assemblyResolvedInputClosure struct {
	owner *assemblyInputOwner
	actor management.ExecutionActorRef
	closure assemblyInputClosureRef
	CAS assemblyCatalogCAS
	source assemblyCatalogSource
	policy assemblyCatalogPolicy
	manifest backend.DocumentImageAssemblyManifest
	records []assemblyCatalogRef
	trust assemblyObservedTrust
	closedLock []byte
	verification []assemblyVerificationFile
	profileSHA256,profileSourceSHA256,toolDomainSHA256 string
}

func (r *assemblyResolvedInputClosure) current(ctx context.Context)error {
	if r==nil||r.owner==nil||ctx==nil||ctx.Err()!=nil||!validAssemblyInputClosureRef(r.closure){return errAssemblyClosureDomain}
	if r.owner.api==nil||r.owner.api.deps.Product==nil{return errAssemblyClosureDomain}
	configuration:=r.owner.api.deps.Product.DocumentRuntimeConfiguration()
	if !configuration.Configured()||!configuration.Valid()||configuration.ProfileSHA256!=r.profileSHA256||configuration.ToolDomainSHA256!=r.toolDomainSHA256{return errAssemblyClosureDomain}
	if err:=r.owner.authorize(ctx,r.actor);err!=nil{return err}
	snapshot,err:=r.owner.readCurrent(ctx);if err!=nil{return err}
	if snapshot.CAS!=r.CAS||snapshot.Source!=r.source||snapshot.State.Policy!=r.policy{return errAssemblyCatalogConflict}
	selected:=map[assemblyCatalogRef]bool{};for _,ref:=range snapshot.Index.Records {selected[ref]=true}
	for _,ref:=range r.records {if !selected[ref]{return errAssemblyCatalogConflict}}
	if err:=checkAssemblyClosureTrust(r.trust,r.policy,time.Now().Unix());err!=nil{return err}
	if r.owner.api.deps.Product.DocumentRuntimeConfiguration()!=configuration{return errAssemblyClosureDomain}
	return r.owner.authorize(ctx,r.actor)
}

func checkAssemblyClosureTrust(trust assemblyObservedTrust,policy assemblyCatalogPolicy,now int64)error {
	if trust.Policy.PolicyRevision!=policy.Revision||trust.Policy.SourcePolicy.SHA256!=policy.Digest||trust.Pins.Product==nil||checkAssemblyDebianSignatures(trust.Signatures,trust.Policy,trust.Keys,trust.Pins,now)!=nil{return errAssemblyClosureDomain}
	return nil
}

func readAssemblyClosureDomain(ctx context.Context,owner *assemblyInputOwner,selected map[assemblyCatalogRef]bool,kind string,ref assemblyCatalogRef)([]json.RawMessage,[]assemblyCatalogRef,error) {
	if !selected[ref]||ref.Kind!="input-closure-domain"{return nil,nil,errAssemblyClosureDomain}
	body,err:=owner.readMetadata(ctx,ref);if err!=nil{return nil,nil,err}
	var domain assemblyClosureDomainV1
	if decodeAssemblyCatalog(body,assemblyCatalogRecordLimit,&domain)!=nil||domain.Schema!=1||domain.RecordType!="luci-document-input-domain-v1"||domain.Kind!=kind||len(domain.Pages)==0||len(domain.Pages)>384{return nil,nil,errAssemblyClosureDomain}
	refs:=[]assemblyCatalogRef{ref};rows:=[]json.RawMessage{};seen:=map[assemblyCatalogRef]bool{};bytes:=0
	for part,pageRef:=range domain.Pages {
		if !selected[pageRef]||pageRef.Kind!="input-closure-domain"||seen[pageRef]{return nil,nil,errAssemblyClosureDomain};seen[pageRef]=true
		data,e:=owner.readMetadata(ctx,pageRef);if e!=nil{return nil,nil,e};bytes+=len(data);if bytes>32<<20{return nil,nil,errAssemblyClosureDomain}
		var page assemblyClosurePageV1
		if decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&page)!=nil||page.Schema!=1||page.RecordType!="luci-document-input-domain-page-v1"||page.Kind!=kind||page.Part!=part||len(page.Rows)==0||len(page.Rows)>256||len(rows)>100000-len(page.Rows){return nil,nil,errAssemblyClosureDomain}
		rows=append(rows,page.Rows...);refs=append(refs,pageRef)
	}
	return rows,refs,nil
}

func decodeAssemblyClosureRows[T any](rows []json.RawMessage)([]T,error) {
	out:=make([]T,len(rows));for i,row:=range rows {if decodeAssemblyCatalog(row,assemblyCatalogRecordLimit,&out[i])!=nil{return nil,errAssemblyClosureDomain}};return out,nil
}

func (o *assemblyInputOwner) resolveCurrentAssemblyClosure(ctx context.Context,actor management.ExecutionActorRef,ref assemblyInputClosureRef)(*assemblyResolvedInputClosure,error) {
	if o==nil||ctx==nil||ctx.Err()!=nil||!validAssemblyInputClosureRef(ref){return nil,errAssemblyClosureDomain}
	if err:=o.authorize(ctx,actor);err!=nil{return nil,err}
	snapshot,err:=o.readCurrent(ctx);if err!=nil{return nil,err}
	selected:=map[assemblyCatalogRef]bool{};var rootRef assemblyCatalogRef
	for _,record:=range snapshot.Index.Records {selected[record]=true;if record.Kind=="input-closure"&&record.Digest==ref.Digest {if rootRef!=(assemblyCatalogRef{}){return nil,errAssemblyClosureDomain};rootRef=record}}
	if rootRef==(assemblyCatalogRef{}){return nil,errAssemblyClosureDomain}
	data,err:=o.readMetadata(ctx,rootRef);if err!=nil{return nil,err}
	var root assemblyClosureRootV1
	if decodeAssemblyCatalog(data,assemblyCatalogRecordLimit,&root)!=nil||root.Schema!=1||root.RecordType!="luci-document-input-closure-v1"||root.Source!=snapshot.Source||root.Policy!=snapshot.State.Policy||root.ExpectedCAS.Revision==0||root.ExpectedCAS.Revision>=snapshot.CAS.Revision||!assemblyCatalogHex(root.ExpectedCAS.Digest)||!backend.ValidateDocumentImageAssemblyManifest(root.Manifest)||len(root.Domains)!=8{return nil,errAssemblyClosureDomain}
	wanted:=[]string{"base-rootfs","debian-binaries","debian-sources","dependency-closure","expected-installed-files","luci-source-helper","notices-and-sources","wheels"}
	rows:=map[string][]json.RawMessage{};records:=[]assemblyCatalogRef{rootRef};totalBytes:=0
	for i,domainRef:=range root.Domains {
		if domainRef.Kind!=wanted[i]{return nil,errAssemblyClosureDomain}
		values,refs,e:=readAssemblyClosureDomain(ctx,o,selected,domainRef.Kind,domainRef.Root);if e!=nil{return nil,e}
		for _,r:=range refs {totalBytes+=int(r.Bytes);if totalBytes>32<<20||len(records)>=assemblyCatalogRefLimit{return nil,errAssemblyClosureDomain};records=append(records,r)}
		rows[domainRef.Kind]=values
	}
	base,err:=decodeAssemblyClosureRows[assemblyObservedBase](rows["base-rootfs"]);if err!=nil||len(base)!=1{return nil,errAssemblyClosureDomain}
	wheels,err:=decodeAssemblyClosureRows[assemblyObservedWheel](rows["wheels"]);if err!=nil{return nil,err}
	sources,err:=decodeAssemblyClosureRows[assemblyObservedSource](rows["debian-sources"]);if err!=nil{return nil,err}
	files,err:=decodeAssemblyClosureRows[assemblyObservedFile](rows["expected-installed-files"]);if err!=nil{return nil,err}
	notices,err:=decodeAssemblyClosureRows[assemblyObservedNotice](rows["notices-and-sources"]);if err!=nil{return nil,err}
	helper,err:=decodeAssemblyClosureRows[assemblyObservedHelper](rows["luci-source-helper"]);if err!=nil||len(helper)!=1{return nil,errAssemblyClosureDomain}
	binaries,err:=decodeAssemblyClosureRows[assemblyClosureBinaryRow](rows["debian-binaries"]);if err!=nil||len(binaries)<17+174||len(binaries)>17+174*4{return nil,errAssemblyClosureDomain}
	for _,row:=range binaries {count:=0;for _,present:=range []bool{row.Root!=nil,row.TrustPolicy!=nil,row.TrustKeys!=nil,row.TrustSignatures!=nil,row.TrustPins!=nil,row.IndexSelection!=nil,row.Part!=nil,row.ClaimPart!=nil}{if present{count++}};if count!=1||row.Part==nil&&len(row.Controls)!=0{return nil,errAssemblyClosureDomain}}
	if binaries[0].Root==nil||binaries[1].TrustPolicy==nil||binaries[2].TrustKeys==nil||binaries[3].TrustSignatures==nil||binaries[4].TrustPins==nil||binaries[5].IndexSelection==nil{return nil,errAssemblyClosureDomain}
	trust:=assemblyObservedTrust{Policy:*binaries[1].TrustPolicy,Keys:*binaries[2].TrustKeys,Signatures:*binaries[3].TrustSignatures,Pins:*binaries[4].TrustPins};if err=checkAssemblyClosureTrust(trust,root.Policy,time.Now().Unix());err!=nil{return nil,err}
	binaryRoot:=binaries[0].Root
	if binaryRoot.Schema!=1||binaryRoot.RecordType!="luci-debian-binary-bindings-v1"||binaryRoot.PackageCount!=174||binaryRoot.PackageBytes!=assemblyDebianBinaryBytes||binaryRoot.DependencyLockSHA256!=root.Source.DependencyLockDigest||binaryRoot.AcquisitionManifestSHA256!=root.Source.InputManifestDigest||len(binaryRoot.Parts)!=11{return nil,errAssemblyClosureDomain}
	claimBytes:=[174][]byte{};claimParts:=[174]int{};previousOrdinal:=-1
	for _,row:=range binaries[17:] {part:=row.ClaimPart;if part==nil||part.Ordinal<0||part.Ordinal>=174||part.Ordinal<previousOrdinal||part.Ordinal>previousOrdinal+1||part.Part!=claimParts[part.Ordinal]{return nil,errAssemblyClosureDomain};raw,e:=base64.StdEncoding.Strict().DecodeString(part.Data);if e!=nil||len(raw)==0||len(raw)>16384||base64.StdEncoding.EncodeToString(raw)!=part.Data||len(claimBytes[part.Ordinal])+len(raw)>65536{return nil,errAssemblyClosureDomain};claimBytes[part.Ordinal]=append(claimBytes[part.Ordinal],raw...);claimParts[part.Ordinal]++;previousOrdinal=part.Ordinal}
	if previousOrdinal!=173{return nil,errAssemblyClosureDomain}
	controls:=[]assemblyDependencyPackage{};packages:=[]backend.DocumentImageAssemblyPackage{}
	for part,row:=range binaries[6:17] {
		if row.Part==nil||row.Part.Part!=part||row.Part.FirstLockOrdinal!=part*16||len(row.Part.Bindings)!=len(row.Controls)||len(row.Controls)!=min(16,174-part*16)||row.Part.SignedIndexes!=binaryRoot.SignedIndexes||row.Part.DependencyLockSHA256!=binaryRoot.DependencyLockSHA256{return nil,errAssemblyClosureDomain}
		canonical,e:=json.Marshal(row.Part);binding:=binaryRoot.Parts[part]
		if e!=nil||binding.Part!=part||binding.FirstLockOrdinal!=part*16||binding.Count!=len(row.Controls)||binding.Object!=(assemblyDebianObjectRef{"binary-binding-part",assemblyCatalogDigest(canonical),int64(len(canonical))}){return nil,errAssemblyClosureDomain}
		for i,control:=range row.Controls {b:=row.Part.Bindings[i];projection,e:=assemblyProjectionFromClaimBytes(claimBytes[part*16+i],b);if e!=nil||!equalAssemblyDebianJSON(projection,control)||b.LockOrdinal!=part*16+i||!assemblyCatalogHex(b.DebObject.SHA256)||b.DebObject.Bytes<=0{return nil,errAssemblyClosureDomain};packages=append(packages,backend.DocumentImageAssemblyPackage{DocumentImageAssemblyInput:backend.DocumentImageAssemblyInput{Path:"debs/"+path.Base(b.Filename),Bytes:b.DebObject.Bytes,Digest:b.DebObject.SHA256},Name:b.Name,Version:b.Version,Architecture:b.Architecture})}
		controls=append(controls,row.Controls...)
	}
	actualBinary,err:=assemblyClosureObject("debian-binaries",binaryRoot);if err!=nil||!selected[actualBinary.ref]||!selected[binaryRoot.SignedIndexes]{return nil,errAssemblyClosureDomain}
	indexBytes,err:=o.readMetadata(ctx,binaryRoot.SignedIndexes);if err!=nil{return nil,err};var indexes assemblyDebianSignedIndexBindingsV1
	if decodeAssemblyCatalog(indexBytes,assemblyCatalogRecordLimit,&indexes)!=nil||checkAssemblyDebianIndexBindings(indexes,trust.Policy,trust.Keys,trust.Signatures,*binaries[5].IndexSelection)!=nil||!selected[indexes.Capture.RetainedManifest]{return nil,errAssemblyClosureDomain}
	records=append(records,actualBinary.ref,binaryRoot.SignedIndexes,indexes.Capture.RetainedManifest)
	for _,entry:=range []struct{ref assemblyCatalogRef;value any}{{indexes.RootPolicy,trust.Policy},{indexes.KeyObservations,trust.Keys},{indexes.ReleaseSignatures,trust.Signatures}} {canonical,e:=assemblyClosureObject(entry.ref.Kind,entry.value);if e!=nil||canonical.ref!=entry.ref||!selected[entry.ref]{return nil,errAssemblyClosureDomain};actual,e:=o.readMetadata(ctx,entry.ref);if e!=nil||string(actual)!=string(canonical.data){return nil,errAssemblyClosureDomain};records=append(records,entry.ref)}
	observed:=&assemblyObservedDomains{base:base[0],wheels:wheels,sources:sources,installed:files,notices:notices,helper:helper[0]}
	dependency,err:=validateAssemblyObservedDomains(observed,root.Source,controls);if err!=nil{return nil,err}
	dependencyRows,err:=decodeAssemblyClosureRows[assemblyClosureDependencyRow](rows["dependency-closure"]);if err!=nil{return nil,err}
	derived:=assemblyDependencyClosureV1{Schema:1,Packages:[]assemblyDependencyPackage{},Edges:[]assemblyDependencyEdge{}};edges,locks:=false,false;closedLock:=[]byte{};lockParts:=0
	for _,row:=range dependencyRows {count:=0;if row.Package!=nil{count++};if row.Edge!=nil{count++};if row.LockPart!=nil{count++};if count!=1{return nil,errAssemblyClosureDomain};if row.LockPart!=nil {locks=true;raw,e:=base64.StdEncoding.Strict().DecodeString(row.LockPart.Data);if e!=nil||row.LockPart.Part!=lockParts||len(raw)==0||len(raw)>16384||base64.StdEncoding.EncodeToString(raw)!=row.LockPart.Data||len(closedLock)+len(raw)>2<<20{return nil,errAssemblyClosureDomain};closedLock=append(closedLock,raw...);lockParts++}else if row.Edge!=nil {if locks{return nil,errAssemblyClosureDomain};edges=true;derived.Edges=append(derived.Edges,*row.Edge)}else{if edges||locks{return nil,errAssemblyClosureDomain};derived.Packages=append(derived.Packages,*row.Package)}}
	if !equalAssemblyDebianJSON(dependency,derived)||!equalAssemblyDebianJSON(packages,root.Manifest.Packages)||root.Manifest.BaseLayer!=observed.base.Layer||root.Manifest.Extractor!=observed.helper.Extractor||root.Manifest.Verifier!=observed.helper.Verifier||root.Manifest.Requirements!=observed.helper.Requirements{return nil,errAssemblyClosureDomain}
	actualWheels:=[]backend.DocumentImageAssemblyInput{};for _,w:=range wheels {actualWheels=append(actualWheels,w.Archive)}
	actualNotices:=[]backend.DocumentImageAssemblyInput{};noticePaths:=map[string]bool{};for _,notice:=range notices {if !noticePaths[notice.File.Path]{noticePaths[notice.File.Path]=true;actualNotices=append(actualNotices,notice.File)}};sort.Slice(actualNotices,func(i,j int)bool{return actualNotices[i].Path<actualNotices[j].Path})
	if !equalAssemblyDebianJSON(root.Manifest.Wheels,actualWheels)||!equalAssemblyDebianJSON(root.Manifest.Notices,actualNotices)||root.Manifest.SourceCommit!=root.Source.SourceCommit||root.Manifest.SourceDigest!=root.Source.SourceDigest||root.Manifest.HelperDigest!=root.Source.HelperDigest||root.Manifest.RecipeDigest!=root.Source.RecipeDigest||root.Manifest.SourceRetentionDigest!=root.Source.SourceRetentionDigest{return nil,errAssemblyClosureDomain}
	verification,err:=deriveAssemblyVerificationFiles(files);if err!=nil{return nil,err};inputs:=[]backend.DocumentImageAssemblyInput{};for _,v:=range verification{inputs=append(inputs,v.input)}
	if !equalAssemblyDebianJSON(inputs,root.Manifest.VerificationInputs)||root.Manifest.DependencyLock!=(backend.DocumentImageAssemblyInput{Path:"dependency-lock.json",Bytes:int64(len(closedLock)),Digest:assemblyCatalogDigest(closedLock)}){return nil,errAssemblyClosureDomain}
	var lock assemblyClosedLockV1;if decodeAssemblyCatalog(closedLock,2<<20,&lock)!=nil||lock.Schema!=1||lock.State!="closed-reviewed"||lock.Unresolved==nil||len(lock.Unresolved)!=0{return nil,errAssemblyClosureDomain}
	original,e:=base64.StdEncoding.Strict().DecodeString(lock.SourceMetadata);if e!=nil||base64.StdEncoding.EncodeToString(original)!=lock.SourceMetadata{return nil,errAssemblyClosureDomain}
	derivedLock,err:=deriveAssemblyClosedLock(original,root.Source,dependency,observed,verification,actualNotices);if err!=nil||string(derivedLock)!=string(closedLock){return nil,errAssemblyClosureDomain}
	if !assemblyCatalogHex(root.ProducerSourceSHA256){return nil,errAssemblyClosureDomain}
	resolved:=&assemblyResolvedInputClosure{owner:o,actor:actor,closure:ref,CAS:snapshot.CAS,source:root.Source,policy:root.Policy,manifest:root.Manifest,records:records,trust:trust,closedLock:closedLock,verification:verification,profileSHA256:root.ProducerProfileSHA256,profileSourceSHA256:root.ProducerSourceSHA256,toolDomainSHA256:root.ToolDomainSHA256}
	if err=resolved.current(ctx);err!=nil{return nil,err};return resolved,nil
}

func (c *assemblyAcceptedInputCandidate) canonicalCopy() []assemblyCatalogObject {
	if c==nil{return nil};c.mu.Lock();defer c.mu.Unlock()
	out:=make([]assemblyCatalogObject,len(c.records));for i,r:=range c.records {out[i]=assemblyCatalogObject{r.ref,append([]byte(nil),r.data...)}}
	return out
}

func (c *assemblyAcceptedInputCandidate) retire()error {
	if c==nil{return errAssemblyClosureDomain}
	c.mu.Lock();c.retiring=true
	if c.publication!=nil&&c.publication.inFlight {c.publication.retired=true;c.publicationRetired=true;c.publicationUnknown=true}
	observed:=c.observed;c.mu.Unlock()
	// Current permission, source selection and catalog eligibility are not
	// prerequisites for closing the original owner. A failed Close keeps the
	// actual owner reachable; it never frees charge by replacing its pointer.
	if observed==nil||observed.domains==nil{return errAssemblyClosureDomain}
	return observed.domains.Close()
}

func (c *assemblyAcceptedInputCandidate) current(ctx context.Context)error {
	if c==nil||ctx==nil||ctx.Err()!=nil{return errAssemblyClosureDomain}
	c.mu.Lock();stale:=c.retiring||c.publicationRetired;owner,p,b,d,expected,source,policy:=c.owner,c.plan,c.binary,c.observed,c.expected,c.source,c.policy;c.mu.Unlock()
	if stale||owner==nil||p==nil||b==nil||d==nil||p.owner!=owner||d.plan!=p||d.binary!=b||b.plan!=p{return errAssemblyClosureDomain}
	if err:=d.current(ctx,owner.api);err!=nil{return err}
	snapshot,err:=owner.readCurrent(ctx);if err!=nil{return err}
	if snapshot.CAS!=expected||snapshot.Source!=source||snapshot.State.Policy!=policy{return errAssemblyCatalogConflict}
	if err=checkAssemblyClosureTrust(d.trust,policy,time.Now().Unix());err!=nil{return err}
	c.mu.Lock();stale=c.retiring||c.publicationRetired;c.mu.Unlock();if stale{return errAssemblyClosureDomain}
	return d.current(ctx,owner.api)
}

func assemblyClosureObject(kind string,value any)(assemblyCatalogObject,error) {
	data,err:=json.Marshal(value);ref:=assemblyCatalogRef{1,kind,assemblyCatalogDigest(data),int64(len(data))}
	if err!=nil||!validAssemblyCatalogRef(ref){return assemblyCatalogObject{},errAssemblyClosureDomain}
	return assemblyCatalogObject{ref,data},nil
}

// Large inventories are finite canonical pages in the existing catalog, never
// a parallel store. The domain root selects every page in exact part order.
func appendAssemblyClosureDomain(records *[]assemblyCatalogObject,kind string,rows []json.RawMessage)(assemblyCatalogRef,error) {
	if len(rows)==0||len(rows)>100000{return assemblyCatalogRef{},errAssemblyClosureDomain}
	pages:=[]assemblyCatalogRef{}
	for at:=0;at<len(rows); {
		end:=at;bytes:=0
		for end<len(rows)&&end-at<256&&bytes+len(rows[end])+1<=48<<10 {bytes+=len(rows[end])+1;end++}
		if end==at||len(pages)>=384{return assemblyCatalogRef{},errAssemblyClosureDomain}
		page:=assemblyClosurePageV1{1,"luci-document-input-domain-page-v1",kind,len(pages),rows[at:end]}
		object,err:=assemblyClosureObject("input-closure-domain",page);if err!=nil{return assemblyCatalogRef{},err}
		*records=append(*records,object);pages=append(pages,object.ref);at=end
	}
	object,err:=assemblyClosureObject("input-closure-domain",assemblyClosureDomainV1{1,"luci-document-input-domain-v1",kind,pages});if err!=nil{return assemblyCatalogRef{},err}
	*records=append(*records,object)
	if len(*records)>assemblyCatalogRefLimit-1{return assemblyCatalogRef{},errAssemblyClosureDomain}
	return object.ref,nil
}

func assemblyClosureRows[T any](values []T)([]json.RawMessage,error) {
	rows:=make([]json.RawMessage,len(values));for i,value:=range values {data,err:=json.Marshal(value);if err!=nil{return nil,err};rows[i]=data};return rows,nil
}

func buildDocumentAssemblyClosure(ctx context.Context,a *API,p *debianProducerPlan,binary *debianBinarySemanticCapture)(*assemblyAcceptedInputCandidate,error) {
	if ctx==nil||ctx.Err()!=nil||a==nil||p==nil||p.owner==nil||binary==nil||binary.plan!=p||binary.spool==nil||p.current(ctx,a)!=nil{return nil,errAssemblyClosureDomain}
	// This concrete constructor is the separately owned actual bounded source,
	// base-rootfs and wheel inspection join. No metadata-only fallback exists.
	domains,err:=captureDocumentAssemblyDomains(ctx,a,p,binary);if err!=nil{return nil,err}
	if domains==nil||domains.plan!=p||domains.binary!=binary||domains.current(ctx,a)!=nil{return nil,errAssemblyClosureDomain}
	snapshot,err:=p.owner.readCurrent(ctx);if err!=nil{return nil,err}
	if snapshot.CAS!=(assemblyCatalogCAS{p.identity.ExpectedCAS.Revision,p.identity.ExpectedCAS.Digest}){return nil,errAssemblyCatalogConflict}
	controls:=make([]assemblyDependencyPackage,0,174)
	if p.binary==nil{return nil,errAssemblyClosureDomain}
	binaryRows:=[]assemblyClosureBinaryRow{{Root:&binary.root},{TrustPolicy:&domains.trust.Policy},{TrustKeys:&domains.trust.Keys},{TrustSignatures:&domains.trust.Signatures},{TrustPins:&domains.trust.Pins},{IndexSelection:&p.binary.indexSelection}}
	for part:=0;part<11;part++ {
		row:=assemblyClosureBinaryRow{Part:&binary.parts[part]}
		if len(row.Part.Bindings)!=min(16,174-part*16){return nil,errAssemblyClosureDomain}
		for i,binding:=range row.Part.Bindings {ordinal:=part*16+i;value,e:=assemblyProjectionFromClaimBytes(binary.claimBytes[ordinal],binding);retained,retainedErr:=assemblyDependencyFromControl(binary.controls[ordinal]);if e!=nil||retainedErr!=nil||!equalAssemblyDebianJSON(value,retained){return nil,errAssemblyClosureDomain};row.Controls=append(row.Controls,value);controls=append(controls,value)}
		binaryRows=append(binaryRows,row)
	}
	for ordinal,raw:=range binary.claimBytes {for at:=0;at<len(raw); {end:=min(at+16384,len(raw));binaryRows=append(binaryRows,assemblyClosureBinaryRow{ClaimPart:&assemblyClosureClaimPart{ordinal,at/16384,base64.StdEncoding.EncodeToString(raw[at:end])}});at=end}}
	dependency,err:=validateAssemblyObservedDomains(domains,snapshot.Source,controls);if err!=nil{return nil,err}
	manifest:=backend.DocumentImageAssemblyManifest{Version:1,Profile:backend.DocumentImageAssemblyProfile,SourceCommit:snapshot.Source.SourceCommit,SourceDigest:snapshot.Source.SourceDigest,HelperDigest:snapshot.Source.HelperDigest,RecipeDigest:snapshot.Source.RecipeDigest,SourceRetentionDigest:snapshot.Source.SourceRetentionDigest,BaseLayer:domains.base.Layer,Extractor:domains.helper.Extractor,Verifier:domains.helper.Verifier,Requirements:domains.helper.Requirements,DependencyLock:domains.helper.DependencyLock}
	for _,tuple:=range binary.expected.Selection.Packages {manifest.Packages=append(manifest.Packages,backend.DocumentImageAssemblyPackage{DocumentImageAssemblyInput:backend.DocumentImageAssemblyInput{Path:"debs/"+path.Base(tuple.Filename),Bytes:tuple.Bytes,Digest:tuple.SHA256},Name:tuple.Name,Version:tuple.Version,Architecture:tuple.Architecture})}
	for _,wheel:=range domains.wheels {manifest.Wheels=append(manifest.Wheels,wheel.Archive)}
	notices:=map[string]backend.DocumentImageAssemblyInput{};for _,notice:=range domains.notices {notices[notice.File.Path]=notice.File}
	for _,notice:=range notices {manifest.Notices=append(manifest.Notices,notice)}
	sort.Slice(manifest.Notices,func(i,j int)bool{return manifest.Notices[i].Path<manifest.Notices[j].Path})
	verification,err:=deriveAssemblyVerificationFiles(domains.installed);if err!=nil{return nil,err};for _,file:=range verification {manifest.VerificationInputs=append(manifest.VerificationInputs,file.input)}
	originalFile,ok:=p.files["dependency-lock"];if !ok{return nil,errAssemblyClosureDomain}
	originalLock,err:=readDebianProductGuard(ctx,p.readBudget,p.inputs,"dependency-lock",assemblyDebianObjectRef{Role:"dependency-lock",SHA256:originalFile.SHA256,Bytes:originalFile.Bytes});if err!=nil{return nil,err}
	closedLock,err:=deriveAssemblyClosedLock(originalLock,snapshot.Source,dependency,domains,verification,manifest.Notices);if err!=nil{return nil,err}
	manifest.DependencyLock=backend.DocumentImageAssemblyInput{Path:"dependency-lock.json",Bytes:int64(len(closedLock)),Digest:assemblyCatalogDigest(closedLock)}
	// These booleans are the result of the real inspection constructor AND the
	// complete cross-domain validators above; neither is decoded as authority.
	manifest.Authenticated,manifest.SBOMClosed,manifest.DistributionNoticesClosed=true,true,true
	if !backend.ValidateDocumentImageAssemblyManifest(manifest){return nil,errAssemblyClosureDomain}
	c:=&assemblyAcceptedInputCandidate{owner:p.owner,plan:p,binary:binary,observed:domains,expected:snapshot.CAS,source:snapshot.Source,policy:snapshot.State.Policy,manifest:manifest,closedLock:closedLock,closedLockInput:manifest.DependencyLock,verification:verification}
	// These are the actual independently captured top-level domains, not the
	// nested closure page format and not a renamed binary evidence-index.
	if p.binary==nil||p.binary.indexManifestRef.Kind!="producer-raw-manifest"||p.binary.indexManifestRef.Digest!=assemblyCatalogDigest(p.binary.indexManifest)||p.binary.indexManifestRef.Bytes!=int64(len(p.binary.indexManifest)){return nil,errAssemblyClosureDomain}
	for _,entry:=range []struct{kind string;value any}{{"signed-index-bindings",binary.indexes},{"debian-binaries",binary.root}} {record,e:=assemblyClosureObject(entry.kind,entry.value);if e!=nil{return nil,e};c.records=append(c.records,record)}
	c.records=append(c.records,assemblyCatalogObject{p.binary.indexManifestRef,append([]byte(nil),p.binary.indexManifest...)})
	dependencyRows:=[]assemblyClosureDependencyRow{}
	for i:=range dependency.Packages {dependencyRows=append(dependencyRows,assemblyClosureDependencyRow{Package:&dependency.Packages[i]})}
	for i:=range dependency.Edges {dependencyRows=append(dependencyRows,assemblyClosureDependencyRow{Edge:&dependency.Edges[i]})}
	for at:=0;at<len(closedLock); {end:=min(at+16384,len(closedLock));dependencyRows=append(dependencyRows,assemblyClosureDependencyRow{LockPart:&assemblyClosureLockPart{Part:at/16384,Data:base64.StdEncoding.EncodeToString(closedLock[at:end])}});at=end}
	root:=assemblyClosureRootV1{Schema:1,RecordType:"luci-document-input-closure-v1",ExpectedCAS:c.expected,Source:c.source,Policy:c.policy,ProducerProfileSHA256:p.profile.SHA256,ProducerSourceSHA256:p.profile.SourceSHA256,ToolDomainSHA256:p.files[assemblyToolDomainRole].SHA256,Manifest:manifest}
	values:=[]struct{kind string;rows []json.RawMessage}{}
	add:=func(kind string,value any)error {data,e:=json.Marshal(value);if e!=nil{return e};var rows []json.RawMessage;if json.Unmarshal(data,&rows)!=nil{return errAssemblyClosureDomain};values=append(values,struct{kind string;rows []json.RawMessage}{kind,rows});return nil}
	if err=add("debian-binaries",binaryRows);err!=nil{return nil,err}
	if err=add("base-rootfs",[]assemblyObservedBase{domains.base});err!=nil{return nil,err}
	if err=add("wheels",domains.wheels);err!=nil{return nil,err}
	if err=add("debian-sources",domains.sources);err!=nil{return nil,err}
	if err=add("expected-installed-files",domains.installed);err!=nil{return nil,err}
	if err=add("notices-and-sources",domains.notices);err!=nil{return nil,err}
	if err=add("luci-source-helper",[]assemblyObservedHelper{domains.helper});err!=nil{return nil,err}
	if err=add("dependency-closure",dependencyRows);err!=nil{return nil,err}
	sort.Slice(values,func(i,j int)bool{return values[i].kind<values[j].kind})
	for _,value:=range values {ref,e:=appendAssemblyClosureDomain(&c.records,value.kind,value.rows);if e!=nil{return nil,e};root.Domains=append(root.Domains,assemblyClosureDomainRef{value.kind,ref})}
	object,err:=assemblyClosureObject("input-closure",root);if err!=nil{return nil,err};c.records=append(c.records,object);c.closure=assemblyInputClosureRef{1,object.ref.Digest}
	if err=c.current(ctx);err!=nil{return nil,err};return c,nil
}
