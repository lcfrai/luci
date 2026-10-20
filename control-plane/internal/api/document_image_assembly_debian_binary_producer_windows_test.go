//go:build windows

package api

import (
 "context"
 "strings"
 "testing"
 "time"

 "golang.org/x/sys/windows"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Lower fixed-argv metadata only. Fabricated handles and call returns cannot
// mint a native run, a Product permit, captured output or a publication claim.
func binaryCommandLowerFixture()(*debianProofNativeOwner,*debianProducerPlan,*debianBinaryNativeTransport,*int){
 now:=time.Now();ticks:=new(int)
 o:=&debianProofNativeOwner{began:now,workDeadline:now.Add(time.Minute),wholeDeadline:now.Add(2*time.Minute)};o.census=newDebianProofNativeCensus(o)
 o.calls=&debianProofWindowsCalls{BinaryTick:func()(uint64,error){*ticks++;return 1000,nil},FileInfo:func(_ windows.Handle,v *windows.ByHandleFileInformation)error{*v=windows.ByHandleFileInformation{NumberOfLinks:1,FileSizeLow:3};return nil}}
 p:=&debianProducerPlan{identity:runtimeinstall.DebianInputProofIdentity{Purpose:runtimeinstall.DebianBinaryInputProofPurpose,RunID:"run"},files:map[string]runtimeinstall.DebianInputProofFile{debianBinaryEntryRole:{Role:debianBinaryEntryRole,Bytes:debianBinaryEntryBytes,SHA256:debianBinaryEntrySHA}},paths:map[string]string{"cp-python":`E:\fixture\python.exe`,debianBinaryEntryRole:`E:\fixture\entry.py`}}
 tr:=&debianBinaryNativeTransport{owner:o,native:&debianBinaryTransportNative{}}
 for i:=range tr.native.aliases{tr.native.aliases[i]=debianBinaryOriginalAlias{handle:windows.Handle(i+100),duplicateReturned:true,borrowReturned:true}}
 tr.selection.request.LoanID="loan";tr.selection.request.ScopeDigest=strings.Repeat("a",64);tr.selection.request.SmallSources[0].Bytes=3;tr.selection.request.SmallSources[0].Digest=strings.Repeat("b",64)
 o.census.transports=[]*debianBinaryNativeTransport{tr};return o,p,tr,ticks
}

func TestBinaryEntryCommandBindsOriginalAliasesSelectionAndRemainingClock(t *testing.T){
 o,p,tr,ticks:=binaryCommandLowerFixture();argv,err:=o.binaryEntryArguments(context.Background(),p,tr)
 if err!=nil||len(argv)!=23||argv[0]!=p.paths["cp-python"]||argv[1]!="-I"||argv[2]!="-S"||argv[3]!="-B"||argv[4]!=p.paths[debianBinaryEntryRole]{t.Fatal("fixed entry bootstrap changed",argv,err)}
 flags:=[]string{"--abi","--operation-id","--loan-id","--source-binding-sha256","--selection-sha256","--selection-bytes","--selection-identity","--work-deadline-tick-ms","--handles"}
 for i,flag:=range flags{if argv[5+2*i]!=flag{t.Fatal("entry ABI flag order changed")}}
 if argv[6]!="luci-debian-binary-entry-v1"||argv[8]!="run"||argv[10]!="loan"||argv[16]!="3"||argv[18]!="1,0,0,0,3,0,0,1"||len(strings.Split(argv[22],","))!=181||*ticks!=1{t.Fatal("original scope/identity/clock was not bound")}
}

func TestBinaryEntryCommandRefusesBeforeClockOnWrongPurposeSourceOrAliases(t *testing.T){
 for _,kind:=range []string{"signature-purpose","wrong-entry","missing-python","alias-return-pending","duplicate-alias","selection-links","expired"}{t.Run(kind,func(t *testing.T){
  o,p,tr,ticks:=binaryCommandLowerFixture()
  switch kind{case "signature-purpose":p.identity.Purpose="";case "wrong-entry":f:=p.files[debianBinaryEntryRole];f.SHA256=strings.Repeat("f",64);p.files[debianBinaryEntryRole]=f;case "missing-python":delete(p.paths,"cp-python");case "alias-return-pending":tr.native.aliases[5].borrowReturned=false;case "duplicate-alias":tr.native.aliases[5].handle=tr.native.aliases[4].handle;case "selection-links":o.calls.FileInfo=func(_ windows.Handle,v *windows.ByHandleFileInformation)error{v.NumberOfLinks=2;v.FileSizeLow=3;return nil};case "expired":o.stopped.Store(true)}
  if _,err:=o.binaryEntryArguments(context.Background(),p,tr);err==nil||*ticks!=0{t.Fatal("invalid original shape sampled clock or admitted command")}
 })}
}

func TestBinaryNewOwnerHasIndependentFixedClocksWithoutResettingSignatureOwner(t *testing.T){
 signature:=newDebianProofNativeOwner(context.Background());binary:=newDebianBinaryProofNativeOwner(context.Background())
 if signature.workDeadline.Sub(signature.began)!=40*time.Second||signature.wholeDeadline.Sub(signature.began)!=60*time.Second||binary.workDeadline.Sub(binary.began)!=300*time.Second||binary.wholeDeadline.Sub(binary.began)!=420*time.Second{t.Fatal("purpose clocks reset or widened legacy owner")}
 if signature.observerStarted.Load()||binary.observerStarted.Load(){t.Fatal("metadata construction started native observation")}
}

func TestBinarySemanticCompletionRequiresOriginalSettledLifetime(t *testing.T){
 // Inert original-handle metadata only; no Windows call is installed/executed.
 for _,boundary:=range []string{"complete","pid","creation","readers","process","handle","job","watchdog","observer","nonzero","unclosed"}{t.Run(boundary,func(t *testing.T){
  now:=time.Now();done:=make(chan struct{});close(done)
  o:=&debianProofNativeOwner{began:now,workDeadline:now.Add(time.Minute),watchDone:done,observer:&debianProofObserver{closed:true}}
  o.census=newDebianProofNativeCensus(o);o.observerStarted.Store(true)
  o.jobs=[]*debianProofJob{{settled:true,ordinal:0}}
  o.lifetimes=[]*debianProofLifetime{{processSettled:true,readersSettled:true,pid:10,creation:20}}
  result:=debianProofChildResult{Ordinal:0,PID:10,CreationTime:20,Started:now,Ended:now,OriginalHandleOwned:true,ExitConfirmed:true,JobEmpty:true,HandlesClosed:true,CaptureSettled:true}
  switch boundary{case "pid":result.PID++;case "creation":result.CreationTime++;case "readers":o.lifetimes[0].readersSettled=false;case "process":o.lifetimes[0].processSettled=false;case "handle":o.lifetimes[0].handles=[]windows.Handle{99};case "job":o.jobs[0].handle=99;case "watchdog":o.watchDone=make(chan struct{});case "observer":o.observer.closed=false;case "nonzero":result.Exit=7;case "unclosed":result.HandlesClosed=false}
  if o.positiveBinaryCompletion(result)!=(boundary=="complete"){t.Fatal("native closure boundary lost",boundary)}
 })}
}
