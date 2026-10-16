package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Test-only positive original owners. Native commands/keys/signatures are never
// executed or parsed. Source fixture bytes are retained exact pin inputs only.
type assemblyDebianFakeGuard struct {
	mu sync.Mutex
	data map[string][]byte
	valid, failClose bool
	closed bool
	reads, closes int
}
func (g *assemblyDebianFakeGuard) CheckCurrent(ctx context.Context) error { g.mu.Lock();defer g.mu.Unlock();if !g.valid || ctx.Err()!=nil{return errAssemblyDebianCapture};return nil }
func (g *assemblyDebianFakeGuard) ReadBounded(ctx context.Context,slot string,max int64)([]byte,error){g.mu.Lock();defer g.mu.Unlock();if !g.valid||ctx.Err()!=nil{return nil,errAssemblyDebianCapture};d,ok:=g.data[slot];if !ok||int64(len(d))>max{return nil,errAssemblyDebianCapture};g.reads++;return append([]byte{},d...),nil}
func (g *assemblyDebianFakeGuard) Transfer(ctx context.Context)(backend.DocumentImageAssemblyEvidenceGuard,error){g.mu.Lock();defer g.mu.Unlock();if !g.valid||ctx.Err()!=nil{return nil,errAssemblyDebianCapture};g.valid=false;return &assemblyDebianFakeGuard{data:g.data,valid:true,failClose:g.failClose},nil}
func (g *assemblyDebianFakeGuard) Close()error{g.mu.Lock();defer g.mu.Unlock();g.closes++;if g.closed{return nil};if !g.valid{return errAssemblyDebianCapture};if g.failClose{return errors.New("synthetic original handle close unknown")};g.valid=false;g.closed=true;return nil}

func assemblyDebianOwnedFixture(t *testing.T)(*assemblyDebianOwnedRun,int64){
	t.Helper()
	policy,keys,signatures,_,now:=assemblyDebianSummaryFixture(t)
	o,actor,source,selected,index:=assemblyCatalogFixture(t,filepath.Join(t.TempDir(),"owner"))
	source.DependencyLockDigest,source.InputManifestDigest=assemblyDebianLockSHA,assemblyDebianManifestSHA
	selected=assemblyCatalogPolicy{policy.PolicyRevision,assemblyDebianPolicySourceSHA}
	index.Records=append(index.Records,assemblyCatalogRef{1,"wheels",assemblyCatalogDigest([]byte("unrelated inventory only")),1})
	cas,err:=o.publishMetadata(context.Background(),actor,assemblyCatalogCAS{},source,selected,index);if err!=nil{t.Fatal(err)}
	r:=&assemblyDebianOwnedRun{owner:o,id:"debian-capture-fixture",actor:actor,expected:cas,source:source,policy:selected,phase:"settled",objects:map[string]assemblyDebianObjectRef{}}
	raw:=map[string][]byte{}
	policyBytes,configBytes:=assemblyDebianSourceFixture(t);raw["policy-source"],raw["producer-config"]=policyBytes,configBytes
	for _,slot:=range []string{"producer-source-receipt","inner-source","projection-source","dependency-lock","acquisition-manifest","snapshot-source","public-key-0","public-key-1","public-key-2","signed-release-0","signed-release-1","signed-release-2","signed-release-3","signed-release-4"}{
		data,e:=os.ReadFile(filepath.Join("testdata","document-assembly-debian","capture-v4",slot+".fixture"));if e!=nil{t.Fatal(e)};raw[slot]=data
	}
	raw["outer-source"]=[]byte("synthetic pinned outer; not executable")
	output:=assemblyDebianRawOutput{Schema:"luci-retained-debian-signature-input-evidence-v2",TrustPolicySHA:assemblyDebianPolicySourceSHA,Consumer:"server-captured AssemblyInputClosureRef; mandatory ValidateCurrent",Missing:[]string{"index-paragraphs","package-source-bindings","installed-manifest","source-retention","notice-closure","output-proof"},MonotonicPolicyState:"not-updated-by-this-read-only-batch"}
	output.Keys=assemblyDebianRawKeyObservation{State:"pinned-official-key-snapshot-identities-derived",ObservedUTC:keys.ObservedAt,UnseenRevocations:"not-established",Records:map[string]assemblyDebianRawKey{}}
	for _,k:=range keys.Keys{expires:=int64(0);if k.Expires!=nil{expires=*k.Expires};output.Keys.Records[k.Fingerprint]=assemblyDebianRawKey{k.Kind,k.Created,expires,strconv.Itoa(k.Algorithm),strconv.Itoa(k.Bits),k.Capabilities,k.SnapshotValidity,k.Fingerprint,k.PrimaryFingerprint}}
	var cfg struct{Releases []struct{Candidates []struct{ID string `json:"id"`;Indexes []assemblyDebianRawIndex `json:"indexCandidates"`} `json:"candidates"`} `json:"releases"`}
	if json.Unmarshal(configBytes,&cfg)!=nil{t.Fatal("source config")}
	for i,s:=range signatures.Releases{
		plain:=[]byte("synthetic plaintext "+strconv.Itoa(i));raw["plaintext-"+strconv.Itoa(i)]=plain
		value:=assemblyDebianRawRelease{InputSHA:s.SignedRelease.SHA256,PlaintextSHA:assemblyCatalogDigest(plain)}
		for _,d:=range s.Dispositions{v:=d.Valid;expires:=int64(0);if v.Expires!=nil{expires=*v.Expires};created:=v.Created;value.Dispositions=append(value.Dispositions,assemblyDebianRawDisposition{State:d.Kind,Signer:v.Signer,Primary:v.Primary,Created:&created,Expires:&expires,HashAlgorithm:strconv.Itoa(v.HashAlgorithm)})}
		for j,l:=range s.Logical{value.Logical=append(value.Logical,assemblyDebianRawLogical{CandidateID:l.CandidateID,ReleaseDate:l.ReleaseDate,ValidUntil:l.ValidUntil,SignatureState:l.SignatureState,Freshness:l.ProducerFreshness,SignedIndexes:cfg.Releases[i].Candidates[j].Indexes,HistoricalTimestamp:l.ProducerHistoricalTimestamp,Era:l.Era,ObservedAt:l.ObservedAt,HistoricalAnchor:l.HistoricalAnchor,UpstreamExpiryState:l.UpstreamExpiryState,HistoricalExpiryState:l.HistoricalExpiryState,KeyRevocationKnowledge:l.KeyRevocationKnowledge,ServerEligibility:l.ServerEligibility})}
		output.Releases=append(output.Releases,value)
	}
	marshal:=func(v any)[]byte{d,e:=json.MarshalIndent(v,"","  ");if e!=nil{t.Fatal(e)};return d}
	raw["producer-output"]=marshal(output)
	inner:=assemblyDebianNativeReceipt{Version:1,Phase:"retained-release-signature-verification",State:"retained-signature-evidence-complete-with-explicit-gaps",SupervisorSHA:assemblyDebianInnerSHA,ConfigSHA:assemblyDebianConfigSHA,PolicySHA:assemblyDebianPolicySourceSHA,Maximum:9,Whole:60,Work:40,CleanupSeconds:20,Within:true,Elapsed:16,Keys:output.Keys,TypedSHA:assemblyCatalogDigest(raw["producer-output"])}
	inner.PDH.Worker=true;inner.PDH.Query.Attempted=true;inner.PDH.Query.Closed=true;inner.PDH.Query.Null=true
	busy:=float64(1)
	for i:=0;i<9;i++{
		kind:="verify";if i<3{kind="dearmor"}else if i==3{kind="keys"}
		r.commands[i]=[]string{"synthetic-never-executed-native-command",strconv.Itoa(i)}
		raw["stdout-"+strconv.Itoa(i)],raw["stderr-"+strconv.Itoa(i)]=[]byte{},[]byte{}
		child:=assemblyDebianNativeChild{Index:i,Kind:kind,Argv:r.commands[i],PID:100+i,Began:float64(now-9+int64(i)),Ended:float64(now-9+int64(i))+.1,ExitConfirmed:true,JobEmpty:true,HandlesClosed:true,BrakeSettled:true,Cleanup:assemblyDebianNativeCleanup{Terminal:true,JobEmpty:true,ClosedHandles:true},StdoutSHA:assemblyCatalogDigest(nil),StderrSHA:assemblyCatalogDigest(nil)}
		child.Admission.Host.Available,child.Admission.Host.CommitAvailable=9<<30,9<<30;child.Admission.CPU.Known=true;child.Admission.CPU.Busy=&busy
		if i>=3{raw["stdout-"+strconv.Itoa(i)]=[]byte("synthetic listing/status "+strconv.Itoa(i));child.StdoutSHA=assemblyCatalogDigest(raw["stdout-"+strconv.Itoa(i)]);child.StdoutBytes=int64(len(raw["stdout-"+strconv.Itoa(i)]))}
		if i<3{raw["keyring-"+strconv.Itoa(i)]=[]byte("synthetic keyring "+strconv.Itoa(i));child.OutputSHA=assemblyCatalogDigest(raw["keyring-"+strconv.Itoa(i)])}
		if i>=4{child.OutputSHA=assemblyCatalogDigest(raw["plaintext-"+strconv.Itoa(i-4)])}
		inner.Children=append(inner.Children,child)
	}
	raw["inner-receipt"]=marshal(inner)
	outer:=assemblyDebianOuterReceipt{Version:1,Phase:"retained-only-debian-signature",SupervisorSHA:assemblyCatalogDigest(raw["outer-source"]),Whole:60,Work:40,CleanupSeconds:20,State:"retained-signature-batch-positive-cleanup",TelemetryLimit:1,TelemetryImports:1,InnerSHA:assemblyCatalogDigest(raw["inner-receipt"]),Within:true,Elapsed:16}
	outer.Children=[]assemblyDebianOuterChild{{Index:0,PID:99,Command:[]string{"synthetic-inner"},ExitConfirmed:true,Capture:true,Watchdog:true,Stdout:assemblyCatalogDigest(nil),Stderr:assemblyCatalogDigest(nil)}}
	outer.Observer=assemblyDebianObserverReceipt{PID:98,Launched:true,Original:true,ObserverOnly:true,MemorySource:"frozen GetPerformanceInfo",ImportsLimit:1,Imports:1,Exit:true,Watchdog:true,Capture:true,Samples:1,Stdout:assemblyCatalogDigest(nil),Stderr:assemblyCatalogDigest(nil)}
	for _,slot:=range []string{"outer-stdout","outer-stderr","observer-stdout","observer-stderr"}{raw[slot]=[]byte{}}
	raw["outer-receipt"]=marshal(outer)
	raw["outer-final"]=marshal(assemblyDebianOuterFinal{State:outer.State,WriteSettled:true,SHA:assemblyCatalogDigest(raw["outer-receipt"]),Elapsed:16,Within:true})
	r.completion=&assemblyDebianOriginalCompletion{runID:r.id,outerSHA:outer.SupervisorSHA,innerReceiptSHA:assemblyCatalogDigest(raw["inner-receipt"]),outerReceiptSHA:assemblyCatalogDigest(raw["outer-receipt"]),finalProjectionSHA:assemblyCatalogDigest(raw["outer-final"]),started:float64(now-15),completed:float64(now+1),originalHandleOwned:true,exitConfirmed:true,capturesSettled:true,watchdogSettled:true,observerSettled:true,publicationSettled:true,handlesSettled:true,innerCommand:outer.Children[0].Command}
	inputs,outputs:=map[string][]byte{},map[string][]byte{}
	for slot,data:=range raw{r.objects[slot]=assemblyDebianObjectRef{slot,assemblyCatalogDigest(data),int64(len(data))};if assemblyDebianInputSlot(slot){inputs[slot]=data}else{outputs[slot]=data}}
	r.inputGuard=&assemblyDebianFakeGuard{data:inputs,valid:true};r.guard=&assemblyDebianFakeGuard{data:outputs,valid:true}
	return r,now
}

func TestAssemblyDebianRawClosedDecoderRejectsDuplicatesUnknownAndTrailing(t *testing.T){
	for _,input:=range []string{`{"state":"a","state":"b"}`,`{"unknown":true}`,`{"state":"a"} {}`,`{"state":"a","receiptWriteSettled":null,"unknownResources":{"x":1,"x":2}}`}{var value assemblyDebianOuterFinal;if decodeAssemblyDebianRaw([]byte(input),&value)==nil{t.Fatal("untrusted raw JSON accepted",input)}}
	var value assemblyDebianOuterFinal;if decodeAssemblyDebianRaw([]byte("{\n  \"state\": \"a\"\n}"),&value)!=nil{t.Fatal("pretty raw JSON rejected as noncanonical")}
}

func TestAssemblyDebianOwnedCaptureConstructAndThreeDomainCAS(t *testing.T){
	r,now:=assemblyDebianOwnedFixture(t);oldGuard:=r.guard
	captured,err:=captureDebianSignatureRun(context.Background(),r);if err!=nil{t.Fatal(err)}
	c,err:=newAssemblyDebianDomains(context.Background(),r.owner,captured,now);if err!=nil{t.Fatal(err)}
	if oldGuard.CheckCurrent(context.Background())==nil || oldGuard.Close()==nil{t.Fatal("old token retained candidate ownership")}
	cas,err:=r.owner.adoptDebianDomainsAt(context.Background(),c,now);if err!=nil || cas.Revision!=r.expected.Revision+1{t.Fatal(cas,err)}
	readback,err:=r.owner.adoptDebianDomainsAt(context.Background(),c,now);if err!=nil || readback!=cas{t.Fatal("same capture repeated publication instead of exact readback",readback,err)}
	current,err:=r.owner.Current(context.Background(),r.actor);if err!=nil{t.Fatal(err)}
	if len(current.Index.Records)!=5{t.Fatal("expected three fact domains and raw manifest plus preserved unrelated inventory",current.Index)}
	for _,ref:=range current.Index.Records{if ref.Kind=="input-closure"{t.Fatal("partial facts selected full closure")}}
	if err=c.retire();err!=nil{t.Fatal(err)}
	if _,err=r.owner.adoptDebianDomainsAt(context.Background(),c,now);err==nil{t.Fatal("retired capability reused")}
}

func TestAssemblyDebianCaptureRequiresActualOwnerSettlementAndCurrentGuard(t *testing.T){
	for _,which:=range []string{"absent-original-owner","observer-unknown","publication-unknown","guard-not-current","extra-slot","wrong-original-receipt"}{t.Run(which,func(t *testing.T){r,_:=assemblyDebianOwnedFixture(t);switch which{case "absent-original-owner":r.completion=nil;case "observer-unknown":r.completion.observerSettled=false;case "publication-unknown":r.completion.publicationSettled=false;case "guard-not-current":r.guard.(*assemblyDebianFakeGuard).valid=false;case "extra-slot":r.objects["arbitrary-audit-path"]=assemblyDebianObjectRef{};case "wrong-original-receipt":r.completion.outerReceiptSHA=assemblyCatalogDigest([]byte("foreign"))};if _,err:=captureDebianSignatureRun(context.Background(),r);err==nil{t.Fatal("receipt bytes minted ownership")};current,err:=r.owner.Current(context.Background(),r.actor);if err!=nil||current.CAS!=r.expected{t.Fatal("failed capture selected evidence",current,err)}})}
}

func TestAssemblyDebianAdoptionCurrentAuthorityCASAndCleanupIndependent(t *testing.T){
	for _,which:=range []string{"installer-revoked","source-cas-changed","guard-invalidated","stale-keys"}{t.Run(which,func(t *testing.T){r,now:=assemblyDebianOwnedFixture(t);captured,err:=captureDebianSignatureRun(context.Background(),r);if err!=nil{t.Fatal(err)};c,err:=newAssemblyDebianDomains(context.Background(),r.owner,captured,now);if err!=nil{t.Fatal(err)};switch which{case "installer-revoked":r.owner.api.deps.AuthorizeRuntimeInstaller=func(context.Context,management.ExecutionActorRef)(management.ExecutionAuthority,error){return management.ExecutionAuthority{},management.ErrExecutionAuthority};case "source-cas-changed":if _,err=r.owner.publishMetadata(context.Background(),r.actor,r.expected,r.source,r.policy,assemblyCatalogIndex{Schema:1,Records:[]assemblyCatalogRef{}});err!=nil{t.Fatal(err)};case "guard-invalidated":c.guard.(*assemblyDebianFakeGuard).valid=false;case "stale-keys":now+=86401};if _,err:=r.owner.adoptDebianDomainsAt(context.Background(),c,now);err==nil{t.Fatal("stale or unavailable candidate selected")};if which!="guard-invalidated"{if err:=c.retire();err!=nil{t.Fatal("revoked eligibility blocked handle retirement",err)}}})}
}

func TestAssemblyDebianPartialCloseRetainsRetirementOnlyOwner(t *testing.T){
	r,_:=assemblyDebianOwnedFixture(t);g:=r.guard.(*assemblyDebianFakeGuard);g.failClose=true
	if r.retire()==nil || r.phase!="retiring" || r.guard!=g{t.Fatal("failed original close lost ownership")}
	if r.current(context.Background())==nil{t.Fatal("failed close permitted validation")}
	g.failClose=false;if r.retire()!=nil || r.phase!="closed"{t.Fatal("exact owned cleanup did not settle")}
}

func TestAssemblyDebianRetirementDoesNotWaitForBlockedEligibility(t *testing.T){
	r,_:=assemblyDebianOwnedFixture(t)
	entered,release,done:=make(chan struct{}),make(chan struct{}),make(chan error,1)
	r.owner.api.deps.AuthorizeRuntimeInstaller=func(context.Context,management.ExecutionActorRef)(management.ExecutionAuthority,error){close(entered);<-release;return management.ExecutionAuthority{},nil}
	go func(){done<-r.current(context.Background())}()
	select{case <-entered:case <-time.After(time.Second):t.Fatal("eligibility fake not entered")}
	if err:=r.retire();err!=nil{close(release);t.Fatal(err)}
	close(release)
	select{case err:=<-done:if err==nil{t.Fatal("late eligibility result revived retired owner")};case <-time.After(time.Second):t.Fatal("eligibility reader did not settle")}
}

func TestAssemblyDebianDistinctOwnedCandidatesHaveOneCatalogCASWinner(t *testing.T){
	left,now:=assemblyDebianOwnedFixture(t);right,_:=assemblyDebianOwnedFixture(t)
	right.owner,right.expected,right.source,right.policy=left.owner,left.expected,left.source,left.policy
	right.id="debian-capture-competing-fixture";right.completion.runID=right.id
	candidates:=[]*assemblyDebianDomainCandidate{}
	for _,r:=range []*assemblyDebianOwnedRun{left,right}{captured,err:=captureDebianSignatureRun(context.Background(),r);if err!=nil{t.Fatal(err)};c,err:=newAssemblyDebianDomains(context.Background(),r.owner,captured,now);if err!=nil{t.Fatal(err)};candidates=append(candidates,c)}
	results:=make(chan error,2);var wg sync.WaitGroup
	for _,candidate:=range candidates{wg.Add(1);go func(c *assemblyDebianDomainCandidate){defer wg.Done();_,err:=left.owner.adoptDebianDomainsAt(context.Background(),c,now);results<-err}(candidate)}
	wg.Wait();close(results);passed,conflicts:=0,0
	for err:=range results{if err==nil{passed++}else if errors.Is(err,errAssemblyCatalogConflict){conflicts++}else{t.Fatal(err)}}
	if passed!=1||conflicts!=1{t.Fatal("two owned candidates bypassed catalog CAS",passed,conflicts)}
	for _,c:=range candidates{if err:=c.retire();err!=nil{t.Fatal(err)}}
}

func TestAssemblyDebianRawStorageRetainedButNotSelectableDomain(t *testing.T){
	r,_:=assemblyDebianOwnedFixture(t)
	data:=[]byte{}
	ref:=assemblyCatalogRef{1,"producer-raw",assemblyCatalogDigest(data),0}
	if err:=r.owner.writeImmutableAssemblyObject(context.Background(),ref,data,true);err!=nil{t.Fatal("first raw storage refused",err)}
	root,err:=r.owner.root();if err!=nil{t.Fatal(err)}
	retained,err:=codexRead(filepath.Dir(root),filepath.Join(root,"records","producer-raw",ref.Digest+".raw"),0)
	if err!=nil||len(retained)!=0{t.Fatal("exact empty raw object not retained/readable",err)}
	if assemblyCatalogKind("producer-raw")||validAssemblyCatalogRef(ref){t.Fatal("raw storage became selectable domain")}
	if err:=r.owner.writeImmutableMetadata(context.Background(),ref,data);err==nil{t.Fatal("metadata writer accepted raw storage domain")}
	if _,err:=r.owner.publishMetadata(context.Background(),r.actor,r.expected,r.source,r.policy,assemblyCatalogIndex{1,[]assemblyCatalogRef{ref}});err==nil{t.Fatal("head selected raw storage instead of typed domain")}
	current,err:=r.owner.Current(context.Background(),r.actor);if err!=nil||current.CAS!=r.expected{t.Fatal("raw storage changed selected head",current,err)}
}
