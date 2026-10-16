package api

import (
 "context"
 "encoding/json"
 "errors"
 "fmt"
 "path/filepath"
 "sync"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
)

const debianProductReadLimit uint64 = 64<<20
const debianProductProjectionReadGrant uint64 = 2182321
var errDebianProductEvidence = errors.New("product original-byte evidence is incomplete or unsettled")

// One parent lifetime debit owner; failure consumes the admitted upper bound.
// The fixed Python read grant is charged before Resume, never refunded/reset.
// This accounts bytes, NOT original native handles or execution eligibility.
type debianProductReadBudget struct {mu sync.Mutex;used uint64;requests uint64;deadline time.Time;poisoned bool}
func(b *debianProductReadBudget)debit(ctx context.Context,bytes,requests uint64)error{
 if b==nil{return errDebianProductEvidence};b.mu.Lock();defer b.mu.Unlock()
 if b.poisoned||ctx.Err()!=nil||!time.Now().Before(b.deadline)||requests==0||b.requests>1024||b.used>debianProductReadLimit||requests>1024-b.requests||bytes>debianProductReadLimit-b.used{b.poisoned=true;return errDebianProductEvidence};b.used+=bytes;b.requests+=requests;return nil
}
func(b *debianProductReadBudget)fail(){if b!=nil{b.mu.Lock();b.poisoned=true;b.mu.Unlock()}}
func(b *debianProductReadBudget)acquisition(ctx context.Context,objects []backend.DocumentImageAssemblyEvidenceObject)error{
 var bytes uint64;for _,o:=range objects{if o.Bytes<0||o.Bytes>16<<20||bytes>debianProductReadLimit-uint64(o.Bytes)-1{return errDebianProductEvidence};bytes+=uint64(o.Bytes)+1};return b.debit(ctx,bytes,uint64(len(objects)))
}
func readDebianProductGuard(ctx context.Context,b *debianProductReadBudget,g backend.DocumentImageAssemblyEvidenceGuard,role string,ref assemblyDebianObjectRef)([]byte,error){
 if g==nil||ref.Bytes<0||ref.Bytes>16<<20||!assemblyCatalogHex(ref.SHA256){return nil,errDebianProductEvidence}
 if e:=b.debit(ctx,uint64(ref.Bytes)+1,1);e!=nil{return nil,e};maximum:=max(int64(1),ref.Bytes)
 data,e:=g.ReadBounded(ctx,role,maximum);if e!=nil||int64(len(data))!=ref.Bytes||assemblyCatalogDigest(data)!=ref.SHA256{b.fail();return nil,errDebianProductEvidence};return data,nil
}
func readDebianProductProduced(ctx context.Context,b *debianProductReadBudget,n *debianProofNativeOwner,path string,maximum int64)([]byte,error){
 if n==nil||maximum<=0||maximum>256<<10{return nil,errDebianProductEvidence};if e:=b.debit(ctx,uint64(maximum)+1,1);e!=nil{return nil,e}
 data,e:=n.readProduced(path,maximum);if e!=nil{b.fail()};return data,e
}

type debianProductProjectionEvidence struct {
 mu sync.Mutex
 identity string
 generation uint64
 phase string
 retiring,moving,closing bool
 acquiring int
 early,late backend.DocumentImageAssemblyEvidenceGuard
 refs map[string]assemblyDebianObjectRef
 lateRoles map[string]bool
 copies []assemblyDebianObjectRef
 request assemblyDebianObjectRef
 budget *debianProductReadBudget
 native *debianProofNativeOwner
}
type debianProductProjectionToken struct {owner *debianProductProjectionEvidence;generation uint64}
func(e *debianProductProjectionEvidence)token()*debianProductProjectionToken{return &debianProductProjectionToken{e,e.generation}}
func(e *debianProductProjectionEvidence)requestRetirement(){e.mu.Lock();e.retiring=true;if e.phase!="closed"{e.phase="retiring"};e.mu.Unlock()}
func(t *debianProductProjectionToken)validLocked()bool{return t!=nil&&t.owner!=nil&&t.generation==t.owner.generation}

// IO is always outside this ownership mutex. Pending acquisition/moves prevent
// positive Close; every nonnil late/partial return is retained in this owner.
func(e *debianProductProjectionEvidence)acquire(ctx context.Context,late bool,objects []backend.DocumentImageAssemblyEvidenceObject)error{
 if err:=e.budget.acquisition(ctx,objects);err!=nil{return err}
 e.mu.Lock();if e.retiring||e.moving||e.closing||e.acquiring!=0||late&&e.phase!="early"||!late&&e.phase!="building"{e.mu.Unlock();return errDebianProductEvidence};e.acquiring++;e.mu.Unlock()
 var guard backend.DocumentImageAssemblyEvidenceGuard;var err error;if e.native==nil{err=errDebianProofCensus}else{guard,err=e.native.acquireEvidenceObjects(ctx,objects)}
 return e.finishAcquisition(late,guard,err)
}
// The exact real acquisition return is joined before its error is classified.
// This lower ownership edge is exercised with inert partial/late tokens only.
func(e *debianProductProjectionEvidence)finishAcquisition(late bool,guard backend.DocumentImageAssemblyEvidenceGuard,err error)error{
 if guard==nil&&err==nil{err=errDebianProductEvidence}
 e.mu.Lock();e.acquiring--;if guard!=nil{if late{e.late=guard}else{e.early=guard}};retired:=e.retiring;token:=e.token();if err==nil&&!retired{if late{e.phase="complete"}else{e.phase="early"}};e.mu.Unlock()
 if err!=nil||retired{_ = token.Close();if err!=nil{return err};return errDebianProductEvidence};return nil
}
func(e *debianProductProjectionEvidence)checkCurrent(ctx context.Context,complete bool)error{
 e.mu.Lock();if e.retiring||e.moving||e.closing||e.acquiring!=0||e.early==nil||e.phase!="early"&&e.phase!="complete"||complete&&(e.phase!="complete"||e.late==nil){e.mu.Unlock();return errDebianProductEvidence};early,late,generation:=e.early,e.late,e.generation;e.mu.Unlock()
 if err:=early.CheckCurrent(ctx);err!=nil{return err};if late!=nil{if err:=late.CheckCurrent(ctx);err!=nil{return err}}
 e.mu.Lock();defer e.mu.Unlock();if e.retiring||e.moving||e.closing||e.generation!=generation||e.early!=early||e.late!=late||ctx.Err()!=nil{return errDebianProductEvidence};return nil
}
func(t *debianProductProjectionToken)CheckCurrent(ctx context.Context)error{
 if t==nil||t.owner==nil{return errDebianProductEvidence};e:=t.owner;e.mu.Lock();valid:=t.validLocked();e.mu.Unlock();if !valid{return errDebianProductEvidence};if err:=e.checkCurrent(ctx,true);err!=nil{return err};e.mu.Lock();defer e.mu.Unlock();if !t.validLocked(){return errDebianProductEvidence};return nil
}
func(t *debianProductProjectionToken)ReadBounded(ctx context.Context,role string,maximum int64)([]byte,error){
 if t==nil||t.owner==nil{return nil,errDebianProductEvidence};e:=t.owner;e.mu.Lock();valid:=t.validLocked()&&!e.retiring&&!e.moving&&!e.closing&&e.phase=="complete";if !valid{e.mu.Unlock();return nil,errDebianProductEvidence};ref,exists:=e.refs[role];guard:=e.early;if e.lateRoles[role]{guard=e.late};e.mu.Unlock()
 if !valid||!exists||maximum<=0||ref.Bytes>maximum{return nil,errDebianProductEvidence};data,err:=readDebianProductGuard(ctx,e.budget,guard,role,ref);if err!=nil{return nil,err};e.mu.Lock();valid=t.validLocked()&&!e.retiring&&!e.moving&&!e.closing;e.mu.Unlock();if !valid{return nil,errDebianProductEvidence};return data,nil
}
func(t *debianProductProjectionToken)Close()error{
 if t==nil||t.owner==nil{return errDebianProductEvidence};e:=t.owner;e.mu.Lock();if !t.validLocked(){e.mu.Unlock();return errDebianProductEvidence};e.retiring=true;if e.phase=="closed"{e.mu.Unlock();return nil};e.phase="retiring";if e.moving||e.closing||e.acquiring!=0{e.mu.Unlock();return errDebianProductEvidence};e.closing=true;early,late:=e.early,e.late;e.mu.Unlock()
 var first,second error;if early!=nil{first=early.Close()};if late!=nil{second=late.Close()}
 e.mu.Lock();if first==nil&&e.early==early{e.early=nil};if second==nil&&e.late==late{e.late=nil};e.closing=false;closed:=e.early==nil&&e.late==nil&&e.acquiring==0&&!e.moving;if closed{e.phase="closed"};e.mu.Unlock();if !closed{return errDebianProductEvidence};return nil
}
func(t *debianProductProjectionToken)Transfer(ctx context.Context)(backend.DocumentImageAssemblyEvidenceGuard,error){
 if t==nil||t.owner==nil{return nil,errDebianProductEvidence};e:=t.owner;e.mu.Lock();if !t.validLocked()||e.retiring||e.moving||e.closing||e.acquiring!=0||e.phase!="complete"||e.early==nil||e.late==nil{e.mu.Unlock();return nil,errDebianProductEvidence};e.moving=true;early,late:=e.early,e.late;e.mu.Unlock()
 fail:=func(err error)(backend.DocumentImageAssemblyEvidenceGuard,error){e.mu.Lock();e.moving=false;e.retiring=true;e.mu.Unlock();_ = t.Close();return nil,err}
 next,err:=early.Transfer(ctx);e.mu.Lock();if next!=nil{e.early=next};retired:=e.retiring;e.mu.Unlock();if err!=nil{return fail(err)};if next==nil||retired||ctx.Err()!=nil{return fail(errDebianProductEvidence)}
 nextLate,err:=late.Transfer(ctx);e.mu.Lock();if nextLate!=nil{e.late=nextLate};retired=e.retiring;e.mu.Unlock();if err!=nil{return fail(err)};if nextLate==nil||retired||ctx.Err()!=nil{return fail(errDebianProductEvidence)}
 if err=next.CheckCurrent(ctx);err!=nil{return fail(err)};if err=nextLate.CheckCurrent(ctx);err!=nil{return fail(err)}
 e.mu.Lock();if !t.validLocked()||e.retiring||e.early!=next||e.late!=nextLate||ctx.Err()!=nil{e.mu.Unlock();return fail(errDebianProductEvidence)};e.generation++;e.moving=false;token:=e.token();e.mu.Unlock();return token,nil
}

func newDebianProductProjectionEvidence(p *debianProducerPlan)*debianProductProjectionEvidence{
 return &debianProductProjectionEvidence{identity:p.identity.RunID,generation:1,phase:"building",refs:map[string]assemblyDebianObjectRef{},lateRoles:map[string]bool{},budget:p.readBudget}
}
func debianProjectionCopyNames()[]struct{role,name string;limit int64}{
 values:=[]struct{role,name string;limit int64}{{"config","config.json",65536},{"snapshot","snapshot.json",4096},{"key-list","key-list.txt",65536}}
 for i:=0;i<5;i++{values=append(values,struct{role,name string;limit int64}{fmt.Sprintf("status-%d",i),fmt.Sprintf("status-%d.txt",i),131072})};for i:=0;i<5;i++{values=append(values,struct{role,name string;limit int64}{fmt.Sprintf("plaintext-%d",i),fmt.Sprintf("plaintext-%d.txt",i),262144})};return values
}
func buildDebianProductProjectionRequest(results []debianProofChildResult,refs []assemblyDebianObjectRef,observed int64)(debianProductProjectionRequest,error){
 request:=debianProductProjectionRequest{Schema:"luci-debian-product-projection-request-v1",Protocol:assemblyDebianProductProtocol,ObservedUnix:observed}
 if len(results)!=9||len(refs)!=13||!assemblyDebianEpoch(observed){return request,errDebianProductEvidence}
 for i,r:=range results{if r.Ordinal!=i||!r.OriginalHandleOwned||!r.ExitConfirmed||!r.JobEmpty||!r.HandlesClosed||!r.CaptureSettled||i<4&&r.Exit!=0||i>=4&&r.Exit!=0&&r.Exit!=2{return request,errDebianProductEvidence};request.Children[i]=debianProductProjectionChild{i,r.Exit}}
 for i,role:=range debianProjectionCopyNames(){r:=refs[i];if r.Role!=role.role||r.Bytes<0||r.Bytes>role.limit||!assemblyCatalogHex(r.SHA256)||r.Bytes==0&&r.SHA256!=assemblyCatalogDigest(nil){return request,errDebianProductEvidence};request.Files=append(request.Files,r)};return request,nil
}

// This actual body is reachable only after the caller has real prerequisites
// and sealed aggregate census. Those capabilities remain absent/refused.
func prepareDebianProductProjection(ctx context.Context,n *debianProofNativeOwner,p *debianProducerPlan,results []debianProofChildResult)error{
 if len(results)!=9||p.inputs==nil{return errDebianProductEvidence}
 e:=newDebianProductProjectionEvidence(p);e.native=n;p.projectionMu.Lock();if p.projection!=nil{p.projectionMu.Unlock();return errDebianProductEvidence};p.projection=e;p.projectionToken=e.token();p.projectionMu.Unlock()
 originals:=map[string][]byte{};objects:=[]backend.DocumentImageAssemblyEvidenceObject{}
 add:=func(role,path string,data []byte){ref:=assemblyDebianObjectRef{role,assemblyCatalogDigest(data),int64(len(data))};e.refs[role]=ref;objects=append(objects,backend.DocumentImageAssemblyEvidenceObject{Role:role,Path:path,Bytes:ref.Bytes,Digest:ref.SHA256})}
 for i,r:=range results{for _,v:=range []struct{prefix string;data []byte}{{"stdout",r.Stdout},{"stderr",r.Stderr}}{role:=fmt.Sprintf("%s-%d",v.prefix,i);add(role,filepath.Join(p.directory,role),v.data)}}
 for i:=0;i<3;i++{role:=fmt.Sprintf("keyring-%d",i);data,err:=readDebianProductProduced(ctx,e.budget,n,filepath.Join(p.directory,role),32<<10);if err!=nil{return err};add(role,filepath.Join(p.directory,role),data)}
 for i:=0;i<5;i++{role:=fmt.Sprintf("plaintext-%d",i);data,err:=readDebianProductProduced(ctx,e.budget,n,filepath.Join(p.directory,role),256<<10);if err!=nil{return err};originals[role]=data;add(role,filepath.Join(p.directory,role),data)}
 for _,v:=range []struct{copy,source string}{{"config","producer-config"},{"snapshot","snapshot-source"}}{f:=p.files[v.source];data,err:=readDebianProductGuard(ctx,e.budget,p.inputs,v.source,assemblyDebianObjectRef{v.source,f.SHA256,f.Bytes});if err!=nil{return err};originals[v.copy]=data}
 originals["key-list"]=results[3].Stdout;for i:=0;i<5;i++{originals[fmt.Sprintf("status-%d",i)]=results[4+i].Stdout}
 for _,copy:=range debianProjectionCopyNames(){data,ok:=originals[copy.role];if !ok||int64(len(data))>copy.limit{return errDebianProductEvidence};if err:=n.writeEvidenceBytes(filepath.Join(p.directory,copy.name),data,int(copy.limit));err!=nil{return err};ref:=assemblyDebianObjectRef{copy.role,assemblyCatalogDigest(data),int64(len(data))};e.copies=append(e.copies,ref);add("projection-copy:"+copy.role,filepath.Join(p.directory,copy.name),data)}
 request,err:=buildDebianProductProjectionRequest(results,e.copies,time.Now().UTC().Unix());if err!=nil{return err};data,err:=json.Marshal(request);if err!=nil||len(data)>64<<10{return errDebianProductEvidence}
 if err=n.writeEvidenceBytes(filepath.Join(p.directory,"projection-request.json"),data,64<<10);err!=nil{return err};add("projection-request",filepath.Join(p.directory,"projection-request.json"),data);e.request=e.refs["projection-request"]
 if len(objects)!=40{return errDebianProductEvidence};if err=e.acquire(ctx,false,objects);err!=nil{return err};if err=e.checkCurrent(ctx,false);err!=nil{return err}
 // Every alias now has a matching original evidence guard, not a hash-only
 // close/reopen gap. Original Job/PI/pipes are still independently owned.
 return n.adoptProduced(ctx,e.early,objects)
}
