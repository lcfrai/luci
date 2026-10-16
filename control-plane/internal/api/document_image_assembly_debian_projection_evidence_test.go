package api

import (
 "bytes"
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "sort"
 "strings"
 "sync"
 "testing"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Pure fixed-wire and fake ownership cases. These do not acquire originals,
// start a producer, query the OS, prove aggregate census or supply eligibility.
func projectionRequestFixture()([]debianProofChildResult,[]assemblyDebianObjectRef){
 children:=make([]debianProofChildResult,9);for i:=range children{children[i]=debianProofChildResult{Ordinal:i,OriginalHandleOwned:true,ExitConfirmed:true,JobEmpty:true,HandlesClosed:true,CaptureSettled:true}}
 refs:=[]assemblyDebianObjectRef{};for _,v:=range debianProjectionCopyNames(){refs=append(refs,assemblyDebianObjectRef{v.role,assemblyCatalogDigest(nil),0})};return children,refs
}
func TestDebianProjectionFixedRequestRejectsMissingReorderedAndUnsettledInputs(t *testing.T){
 children,refs:=projectionRequestFixture();children[8].Exit=2;value,e:=buildDebianProductProjectionRequest(children,refs,1700000000);if e!=nil{t.Fatal(e)};raw,_:=json.Marshal(value)
 for _,key:=range []string{`"path"`,`"argv"`,`"environment"`,`"originalHandleOwned"`}{if bytes.Contains(raw,[]byte(key)){t.Fatal("request acquired ambient launch authority")}}
 for _,mutate:=range []func([]debianProofChildResult,[]assemblyDebianObjectRef){func(c []debianProofChildResult,r []assemblyDebianObjectRef){c[3].Exit=2},func(c []debianProofChildResult,r []assemblyDebianObjectRef){c[4].Exit=1},func(c []debianProofChildResult,r []assemblyDebianObjectRef){c[2].HandlesClosed=false},func(c []debianProofChildResult,r []assemblyDebianObjectRef){c[8].Ordinal=0},func(c []debianProofChildResult,r []assemblyDebianObjectRef){r[0],r[1]=r[1],r[0]},func(c []debianProofChildResult,r []assemblyDebianObjectRef){r[0].Bytes=65537},func(c []debianProofChildResult,r []assemblyDebianObjectRef){r[0].SHA256=strings.Repeat("a",64)}}{
  c,r:=append([]debianProofChildResult(nil),children...),append([]assemblyDebianObjectRef(nil),refs...);mutate(c,r);if _,e=buildDebianProductProjectionRequest(c,r,1700000000);e==nil{t.Fatal("invalid fixed request accepted")}
 }
 if _,e=buildDebianProductProjectionRequest(children[:8],refs,1700000000);e==nil{t.Fatal("missing crypto result accepted")};if _,e=buildDebianProductProjectionRequest(children,refs[:12],1700000000);e==nil{t.Fatal("missing copy accepted")}
}
func TestDebianProjectionProductProfileReservesFortyFourObjectsWithoutChangingLegacy(t *testing.T){
 p:=producerProfileFixture(t);for i:=range p.Files{if p.Files[i].Role=="projection-source"{p.Files[i].Bytes=15522};if p.Files[i].Role=="producer-config"{p.Files[i].SHA256=strings.Repeat("d",64)}}
 for len(p.Files)<211{path:=fmt.Sprintf("support/%03d.bin",len(p.Files));p.Files=append(p.Files,runtimeinstall.DebianInputProofFile{Role:"support:"+path,RelativePath:path,Bytes:1,SHA256:strings.Repeat("a",64)})}
 sort.Slice(p.Files,func(i,j int)bool{return p.Files[i].Role<p.Files[j].Role});raw,_:=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(raw);if _,e:=validateDebianProductProfile(p);e!=nil{t.Fatal(e)}
 path:="support/extra.bin";p.Files=append(p.Files,runtimeinstall.DebianInputProofFile{Role:"support:"+path,RelativePath:path,Bytes:1,SHA256:strings.Repeat("a",64)});sort.Slice(p.Files,func(i,j int)bool{return p.Files[i].Role<p.Files[j].Role});raw,_=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(raw)
 if _,e:=validateDebianProductProfile(p);e==nil{t.Fatal("profile plus44 outputs exceeded256")};if _,e:=validateDebianProducerProfileShape(p,true);e!=nil{t.Fatal("test failed outside product object count")}
 legacy:=producerProfileFixture(t);if _,e:=validateDebianProducerProfile(legacy);e!=nil{t.Fatal("legacy shape changed")};if _,e:=debianProducerProfileBeforeReservation(p);e==nil{t.Fatal("count supplied missing native capability")}
}
func TestDebianProjectionReadDebitIsSharedStickyAndPrecedesRead(t *testing.T){
 b:=&debianProductReadBudget{deadline:time.Now().Add(time.Minute)};g:=&assemblyDebianFakeGuard{valid:true,data:map[string][]byte{"a":[]byte("a")}}
 if _,e:=readDebianProductGuard(context.Background(),b,g,"a",assemblyDebianObjectRef{"a",assemblyCatalogDigest([]byte("a")),1});e!=nil{t.Fatal(e)}
 if b.used!=2||b.requests!=1{t.Fatal("read did not reserve upper bound first")};b.mu.Lock();b.used=debianProductReadLimit-1;b.mu.Unlock();if _,e:=readDebianProductGuard(context.Background(),b,g,"a",assemblyDebianObjectRef{"a",assemblyCatalogDigest([]byte("a")),1});e==nil{t.Fatal("budget overflow accepted")}
 if g.reads!=1||!b.poisoned{t.Fatal("overflow read happened or failed budget was reset")};if e:=b.debit(context.Background(),0,1);e==nil{t.Fatal("later small read cleared failed budget")}
 failed:=&debianProductReadBudget{deadline:time.Now().Add(time.Minute)};if _,e:=readDebianProductGuard(context.Background(),failed,g,"missing",assemblyDebianObjectRef{"missing",assemblyCatalogDigest([]byte("x")),1});e==nil{t.Fatal("failed fake IO passed")};if failed.used!=2||!failed.poisoned{t.Fatal("failed IO refunded admitted bytes")}
 calls:=&debianProductReadBudget{deadline:time.Now().Add(time.Minute)};if e:=calls.debit(context.Background(),1,1024);e!=nil{t.Fatal(e)};if e:=calls.debit(context.Background(),0,1);e==nil{t.Fatal("read call bound accepted overflow")}
}
func projectionTokenFixture()(*debianProductProjectionEvidence,*debianProductProjectionToken){
 b:=&debianProductReadBudget{deadline:time.Now().Add(time.Minute)};early:=&assemblyDebianFakeGuard{valid:true,data:map[string][]byte{"early":[]byte("e")}};late:=&assemblyDebianFakeGuard{valid:true,data:map[string][]byte{"late":[]byte("l")}}
 e:=&debianProductProjectionEvidence{identity:"synthetic",generation:1,phase:"complete",early:early,late:late,refs:map[string]assemblyDebianObjectRef{"early":{"early",assemblyCatalogDigest([]byte("e")),1},"late":{"late",assemblyCatalogDigest([]byte("l")),1}},lateRoles:map[string]bool{"late":true},budget:b};return e,e.token()
}
func TestDebianProjectionTwoGuardsTransferWithoutBudgetResetOrStaleClose(t *testing.T){
 e,token:=projectionTokenFixture();for _,role:=range []string{"early","late"}{if _,err:=token.ReadBounded(context.Background(),role,1);err!=nil{t.Fatal(err)}};next,err:=token.Transfer(context.Background());if err!=nil{t.Fatal(err)}
 if e.budget.used!=4||e.budget.requests!=2||token.Close()==nil||token.CheckCurrent(context.Background())==nil{t.Fatal("move reset debit or stale token retained ownership")}
 if _,err=next.ReadBounded(context.Background(),"late",1);err!=nil{t.Fatal(err)};if e.budget.used!=6{t.Fatal("moved read bypassed original budget")};if next.Close()!=nil{t.Fatal("current exact owners could not retire")}
}
func TestDebianProjectionFailedCloseRetainsOnlyExactUnsettledGuard(t *testing.T){
 e,token:=projectionTokenFixture();early:=e.early.(*assemblyDebianFakeGuard);early.failClose=true;if token.Close()==nil{t.Fatal("failed guard close labelled positive")};if e.early!=early||e.late!=nil||e.phase!="retiring"{t.Fatal("partial closure lost ownership")};early.failClose=false;if token.Close()!=nil||e.phase!="closed"{t.Fatal("retained exact guard could not settle")}
}
func TestDebianProjectionAcquisitionReturnRetainsLatePartialOwnership(t *testing.T){
 // The actual helper used immediately after concrete Acquire; this does not
 // pretend to exercise native acquisition or mint its missing census token.
 e:=&debianProductProjectionEvidence{generation:1,phase:"building",acquiring:1};token:=e.token();e.requestRetirement();if token.Close()==nil{t.Fatal("pending original acquisition labelled closed")}
 late:=&assemblyDebianFakeGuard{valid:true,failClose:true};if e.finishAcquisition(false,late,errors.New("synthetic partial native acquisition"))==nil{t.Fatal("retired partial acquisition accepted")};if e.early!=late||e.phase!="retiring"||e.acquiring!=0{t.Fatal("late partial token was lost")};late.failClose=false;if token.Close()!=nil{t.Fatal("exact late partial token failed retirement")}
}
type projectionBlockedMove struct{assemblyDebianFakeGuard;entered,release chan struct{};late backend.DocumentImageAssemblyEvidenceGuard;err error}
func(g *projectionBlockedMove)Transfer(context.Context)(backend.DocumentImageAssemblyEvidenceGuard,error){g.mu.Lock();g.valid=false;g.mu.Unlock();close(g.entered);<-g.release;return g.late,g.err}
func TestDebianProjectionBlockedTransferRetirementRetainsLatePartialToken(t *testing.T){
 e,token:=projectionTokenFixture();late:=&assemblyDebianFakeGuard{valid:true};blocked:=&projectionBlockedMove{assemblyDebianFakeGuard:assemblyDebianFakeGuard{valid:true},entered:make(chan struct{}),release:make(chan struct{}),late:late,err:errors.New("synthetic partial move")};e.early=blocked
 var once sync.Once;release:=func(){once.Do(func(){close(blocked.release)})};defer release();done:=make(chan error,1);go func(){_,err:=token.Transfer(context.Background());done<-err}()
 select{case<-blocked.entered:case<-time.After(time.Second):t.Fatal("fake move not entered")};retired:=make(chan struct{});go func(){e.requestRetirement();_ = token.Close();close(retired)}();select{case<-retired:case<-time.After(time.Second):t.Fatal("native IO held retirement mutex")};release()
 select{case err:=<-done:if err==nil{t.Fatal("late move promoted retired owner")};case<-time.After(time.Second):t.Fatal("late move failed to join")};late.mu.Lock();closed:=late.closed;late.mu.Unlock();if !closed||e.phase!="closed"{t.Fatal("partial late token lost retirement owner")}
}
func TestDebianProjectionCaptureReconcilesCopiesRequestAndOriginalBytes(t *testing.T){
 children,refs:=projectionRequestFixture();now:=time.Now();raw:=map[string][]byte{};sources:=[]string{"producer-config","snapshot-source","stdout-3"};for i:=0;i<5;i++{sources=append(sources,fmt.Sprintf("stdout-%d",4+i))};for i:=0;i<5;i++{sources=append(sources,fmt.Sprintf("plaintext-%d",i))};for _,role:=range sources{raw[role]=[]byte{}}
 request,err:=buildDebianProductProjectionRequest(children,refs,now.Unix());if err!=nil{t.Fatal(err)};requestBytes,_:=json.Marshal(request);raw["projection-request"]=requestBytes;requestRef:=assemblyDebianObjectRef{"projection-request",assemblyCatalogDigest(requestBytes),int64(len(requestBytes))}
 tenth:=debianProofChildResult{Ordinal:9,Started:now};all:=append(append([]debianProofChildResult(nil),children...),tenth);e:=&debianProductProjectionEvidence{phase:"complete",copies:refs,request:requestRef};completion:=&debianProductOriginalCompletion{native:&debianProofNativeOwner{began:now},children:all,projection:e};receipt:=debianProductOwnerReceipt{Children:all,Started:now.Round(0),Completed:now.Add(time.Second),ProjectionRequest:requestRef,ProjectionInputs:refs}
 if err=checkDebianProductProjectionCapture(raw,receipt,completion);err!=nil{t.Fatal(err)};raw["stdout-3"]=[]byte("same role different retained original");if checkDebianProductProjectionCapture(raw,receipt,completion)==nil{t.Fatal("copied listing no longer matched original bytes")};raw["stdout-3"]=[]byte{}
 request.ObservedUnix=now.Add(time.Hour).Unix();changed,_:=json.Marshal(request);raw["projection-request"]=changed;e.request=assemblyDebianObjectRef{"projection-request",assemblyCatalogDigest(changed),int64(len(changed))};receipt.ProjectionRequest=e.request;if checkDebianProductProjectionCapture(raw,receipt,completion)==nil{t.Fatal("self-consistent request escaped owner clock envelope")}
}
