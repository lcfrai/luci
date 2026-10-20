package product

import (
 "context"
 "strings"
 "testing"

 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func documentAssemblyJobFixture()(*Server,runtimeinstall.DocumentAssemblyArtifact){
 selection:=runtimeinstall.DocumentAssemblyArtifact{InputClosureSHA256:strings.Repeat("a",64),ProfileSHA256:strings.Repeat("b",64),SourceSHA256:strings.Repeat("c",64)}
 s:=&Server{runtimeJob:RuntimeJob{ID:"assembly-job",Status:"installing",OperationKind:runtimeinstall.DocumentRuntimeAssemblyMode,Execution:&runtimeinstall.Outcome{Identity:runtimeinstall.Identity{JobID:"assembly-job",ImageTag:"luci/document-extractor:assembly"},DocumentAssembly:&selection}}}
 return s,selection
}

func TestDocumentAssemblyCapturedJobRefusesCancellationReplacementAndChangedClosure(t *testing.T){
 for _,name:=range []string{"current","cancelled","replaced","sealed","closed","finished","closure"}{t.Run(name,func(t *testing.T){
  s,selection:=documentAssemblyJobFixture()
  switch name{case "cancelled":s.runtimeJob.CancelRequested=true;case "replaced":s.runtimeJob.ID="another-job";case "sealed":s.runtimeInstall.sealed=true;case "closed":s.runtimeInstall.closed=true;case "finished":s.runtimeJob.Status="assembled";case "closure":selection.InputClosureSHA256=strings.Repeat("d",64)}
  err:=s.CheckDocumentAssemblyCurrent(context.Background(),"assembly-job",selection)
  if (err==nil)!=(name=="current"){t.Fatalf("job authority %s: %v",name,err)}
 })}
}

func TestDocumentAssemblyPublicationNeverGrantsRuntimeReadinessOrChangesCapturedSource(t *testing.T){
 for _,name:=range []string{"settled-artifact","runtime-ready","held","not-retired","changed-source","missing-receipt","oversize"}{t.Run(name,func(t *testing.T){
  s,selection:=documentAssemblyJobFixture();s.stateDir=t.TempDir()
  selection.Ref=strings.Repeat("1",64);selection.CaptureSHA256=strings.Repeat("2",64);selection.ReceiptSHA256=strings.Repeat("3",64);selection.CaptureBytes=123
  out:=runtimeinstall.Outcome{Identity:s.runtimeJob.Execution.Identity,Phase:"assembled",CleanupConfirmed:true,DocumentAssembly:&selection}
  switch name{case "runtime-ready":out.Phase="ready";out.ImageID="sha256:"+strings.Repeat("4",64);case "held":out.ResourcesHeld=true;case "not-retired":out.CleanupConfirmed=false;case "changed-source":selection.SourceSHA256=strings.Repeat("5",64);case "missing-receipt":selection.ReceiptSHA256="";case "oversize":selection.CaptureBytes=(2<<30)+1}
  err:=s.publishRuntimeInstall("assembly-job",out)
  if (err==nil)!=(name=="settled-artifact"){t.Fatalf("artifact disposition %s: %v",name,err)}
  if name=="settled-artifact"{if s.runtimeJob.Status!="assembled"||s.runtimeJob.Execution.ImageID!=""{t.Fatal("maintainer artifact became runtime-ready")}}
 })}
}
