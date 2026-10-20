package api

import (
 "context"
 "os"
 "path/filepath"
 "strings"
 "testing"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func TestDocumentAssemblyStageWritesExactGeneratedInputsWithoutOverwrite(t *testing.T){
 root:=t.TempDir();live:=&liveDocumentAssemblyStage{};data:=[]byte("derived-closed-lock\n")
 if e:=writeAssemblyStageFile(context.Background(),live,root,"verification/installed-0000.json",nil,int64(len(data)),assemblyCatalogDigest(data),data);e!=nil{t.Fatal(e)}
 path:=filepath.Join(root,"verification","installed-0000.json");got,e:=os.ReadFile(path);if e!=nil||string(got)!=string(data){t.Fatal("exact generated source not retained",e)}
 if e=writeAssemblyStageFile(context.Background(),live,root,"verification/installed-0000.json",nil,int64(len(data)),assemblyCatalogDigest(data),data);e==nil{t.Fatal("existing staging bytes overwritten")}
 if live.writing||live.writerUnknown{t.Fatal("a returned positive file close retained an active writer")}
}

func TestDocumentAssemblyStageRefusesEscapesWrongDigestAndCancelledWrites(t *testing.T){
 for _,name:=range []string{"escape","wrong-digest","cancelled","retiring"}{t.Run(name,func(t *testing.T){
  root:=t.TempDir();live:=&liveDocumentAssemblyStage{};ctx,cancel:=context.WithCancel(context.Background());defer cancel();path:="payload";digest:=assemblyCatalogDigest([]byte("payload"))
  switch name{case "escape":path="../outside";case "wrong-digest":digest=strings.Repeat("a",64);case "cancelled":cancel();case "retiring":live.retiring=true}
  if e:=writeAssemblyStageFile(ctx,live,root,path,nil,7,digest,[]byte("payload"));e==nil{t.Fatal("unusable source became staged input")}
  if live.writing{t.Fatal("synchronous writer remained active after return")}
 })}
}

func TestDocumentAssemblyRetirementCannotInventReturnOfOriginalAcquisitionOrWriter(t *testing.T){
 for _,name:=range []string{"opening","writing","unknown-close"}{t.Run(name,func(t *testing.T){
  live:=&liveDocumentAssemblyStage{};switch name{case "opening":live.opening=true;case "writing":live.writing=true;case "unknown-close":live.writerUnknown=true}
  g:=&executionAdmission{}
  if e:=g.closeDocumentAssemblyStage(context.Background(),documentImageAssemblyLease{},live);e==nil||!live.retiring{t.Fatal("deadline/nil-owner falsely settled an original operation")}
 })}
}

func TestDocumentAssemblyStagingLeaseBindsCurrentJobAndSeparateDemand(t *testing.T){
 g,req,f,h:=assemblySharedFixture(t);row:=assemblySharedAdmit(t,g,req,f,h)
 spec:=req.Spec;spec.Identity=row.Identity
 selection:=runtimeinstall.DocumentAssemblyArtifact{InputClosureSHA256:row.InputClosureRef.Digest,ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 row.Spec=&spec;row.DocumentSelection=&selection;row.DocumentJobID="assembly-job";row.State="staging"
 demand:=backend.DocumentDomainInputDemand{Inputs:1,OriginalHandles:3,DeclaredBytes:1,ReadCalls:backend.DocumentDomainInputReadCalls,ReadBytes:backend.DocumentDomainInputReadBytes,BlockBytes:backend.DocumentDomainInputBlockBytes,ReadBufferBytes:2*backend.DocumentDomainInputBlockBytes,AcquisitionWorkers:1,RetirementWorkers:1,CancellationWorkers:1,CancellationCallbacks:3}
 row.Stage=&documentAssemblyStageBinding{InputsDigest:strings.Repeat("c",64),Demand:demand,DirectoryBytes:1,State:"reserved",WriterHandles:1,WriterBufferBytes:64<<10}
 if !validDocumentImageAssemblyLease(row){t.Fatal("closed staging row refused")}
 for _,name:=range []string{"missing-job","caller-artifact","opened-without-charge","writer","past-stage"}{t.Run(name,func(t *testing.T){
  copy:=row;binding:=*row.Stage;selected:=selection;copy.Stage=&binding;copy.DocumentSelection=&selected
  switch name{case "missing-job":copy.DocumentJobID="";case "caller-artifact":selected.Ref=strings.Repeat("d",64);case "opened-without-charge":binding.Demand.OriginalHandles=0;case "writer":binding.WriterHandles=2;case "past-stage":copy.State="reserved"}
  if validDocumentImageAssemblyLease(copy){t.Fatal("uncaptured or unretired staging authority accepted")}
 })}
}
