package api

import(
 "context"
 "encoding/json"
 "os"
 "path/filepath"
 "sort"
 "strings"
 "testing"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/product"
 "github.com/lcfrai/luci/control-plane/internal/resourcebudget"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func producerProfileFixture(t *testing.T)runtimeinstall.BundledDebianInputProofProfile{
 t.Helper();p:=runtimeinstall.BundledDebianInputProofProfile{ID:"fixture",Protocol:runtimeinstall.DebianInputProofProtocol,SourceSHA256:strings.Repeat("b",64),Root:t.TempDir()}
 roles:=[]string{"cp-python","gpg","gpgv","projection-entry","projection-source","producer-source-receipt","producer-config","policy-source","dependency-lock","acquisition-manifest","snapshot-source","catalog-source-descriptor","public-key-0","public-key-1","public-key-2","signed-release-0","signed-release-1","signed-release-2","signed-release-3","signed-release-4"};sort.Strings(roles)
 for _,role:=range roles{digest:=strings.Repeat("a",64);switch role{case "projection-source":digest=assemblyDebianProjectionSHA;case "producer-source-receipt":digest=p.SourceSHA256;case "producer-config":digest=assemblyDebianConfigSHA;case "gpg":digest="a5140c85353e8399da8d6bd7e3741524cd76a4e69be88af12042b1c9ef022984";case "gpgv":digest="f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b"};p.Files=append(p.Files,runtimeinstall.DebianInputProofFile{Role:role,RelativePath:"inputs/"+role,Bytes:1,SHA256:digest})};data,_:=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(data);return p
}
func TestDebianProducerProfileCanonicalPinsPathsAndBudget(t *testing.T){
 p:=producerProfileFixture(t);if _,e:=validateDebianProducerProfile(p);e!=nil{t.Fatal(e)}
 for _,mutation:=range []func(*runtimeinstall.BundledDebianInputProofProfile){func(p *runtimeinstall.BundledDebianInputProofProfile){p.ID="../escape"},func(p *runtimeinstall.BundledDebianInputProofProfile){p.Files[0].RelativePath="."},func(p *runtimeinstall.BundledDebianInputProofProfile){p.Files[0].RelativePath="../escape"},func(p *runtimeinstall.BundledDebianInputProofProfile){p.Files[0].Role="arbitrary-helper"},func(p *runtimeinstall.BundledDebianInputProofProfile){p.Files[0].Bytes=16<<20+1},func(p *runtimeinstall.BundledDebianInputProofProfile){p.Files[1].RelativePath=p.Files[0].RelativePath}}{
  bad:=p;bad.Files=append([]runtimeinstall.DebianInputProofFile(nil),p.Files...);mutation(&bad);data,_:=canonicalDebianProducerProfile(bad);bad.SHA256=assemblyCatalogDigest(data);if _,e:=validateDebianProducerProfile(bad);e==nil{t.Fatal("bad canonical descriptor accepted")}
 }
 bad:=p;bad.Files=append([]runtimeinstall.DebianInputProofFile(nil),p.Files...);bad.Files[0].Bytes++ // missing repin
 if _,e:=validateDebianProducerProfile(bad);e==nil{t.Fatal("changed descriptor gained authority from its own fields")}
}
func TestDebianProducerLegacyToolPinsDoNotAdmitNewProductTools(t *testing.T){
 p:=producerProfileFixture(t);for i:=range p.Files{if p.Files[i].Role=="gpg"{p.Files[i].SHA256=strings.Repeat("f",64)}};data,_:=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(data)
 if _,e:=validateDebianProducerProfile(p);e==nil{t.Fatal("registry syntax replaced exact diagnostic tool semantics")}
}
func TestDebianProducerCallerBoundaryRejectsDiagnosticConfigAndReceipt(t *testing.T){
 // This is the exact closed profile boundary called by proveBundledDebianInputs
 // before any owner registration. It does not pretend to package a profile or
 // exercise Product's intentionally empty application registry.
 legacy:="13a80135f8b3b6392d8dc69df5f441a34eae0aa9592cc94dc6710ef2e061d1e0"
 p:=producerProfileFixture(t)
 if _,e:=debianProducerProfileBeforeReservation(p);e==nil{t.Fatal("legacy645 config entered the Go owner")}
 for i:=range p.Files{if p.Files[i].Role=="producer-config"{p.Files[i].SHA256=strings.Repeat("e",64)}}
 p.SourceSHA256=legacy;for i:=range p.Files{if p.Files[i].Role=="producer-source-receipt"{p.Files[i].SHA256=legacy}}
 raw,_:=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(raw)
 if _,e:=debianProducerProfileBeforeReservation(p);e==nil{t.Fatal("legacy13a receipt entered the Go owner")}
 p.SourceSHA256=strings.Repeat("b",64);raw,_=canonicalDebianProducerProfile(p);p.SHA256=assemblyCatalogDigest(raw)
 if _,e:=debianProducerProfileBeforeReservation(p);e==nil{t.Fatal("legacy receipt role was relabelled by another sourceSHA")}
}
func TestDebianProducerCatalogSourceBindsCanonicalCurrentSelection(t *testing.T){
 _,_,source,_,_:=assemblyCatalogFixture(t,filepath.Join(t.TempDir(),"unused"))
 source.DependencyLockDigest,source.InputManifestDigest=assemblyDebianLockSHA,assemblyDebianManifestSHA
 raw,_:=json.Marshal(source);file:=runtimeinstall.DebianInputProofFile{Role:"catalog-source-descriptor",Bytes:int64(len(raw)),SHA256:assemblyCatalogDigest(raw)}
 snap:=assemblyInputCatalogSnapshot{Source:source,State:assemblyInputCatalogState{SourceDescriptor:assemblyCatalogRef{1,"source-descriptor",file.SHA256,file.Bytes},Policy:assemblyCatalogPolicy{Revision:"fixture",Digest:assemblyDebianPolicySourceSHA}}}
 if e:=checkDebianProducerSourceDescriptor(raw,file,snap);e!=nil{t.Fatal(e)}
 stale:=snap;stale.Source.SourceDigest=strings.Repeat("7",64);if checkDebianProducerSourceDescriptor(raw,file,stale)==nil{t.Fatal("different current source accepted")}
 stale=snap;stale.State.SourceDescriptor.Digest=strings.Repeat("8",64);if checkDebianProducerSourceDescriptor(raw,file,stale)==nil{t.Fatal("different selected descriptor accepted")}
 for _,field:=range []string{"lock","manifest"}{bad:=source;if field=="lock"{bad.DependencyLockDigest=strings.Repeat("9",64)}else{bad.InputManifestDigest=strings.Repeat("9",64)};badRaw,_:=json.Marshal(bad);badFile:=file;badFile.Bytes,badFile.SHA256=int64(len(badRaw)),assemblyCatalogDigest(badRaw);badSnap:=snap;badSnap.Source=bad;badSnap.State.SourceDescriptor.Digest,badSnap.State.SourceDescriptor.Bytes=badFile.SHA256,badFile.Bytes;if checkDebianProducerSourceDescriptor(badRaw,badFile,badSnap)==nil{t.Fatal("self-consistent descriptor substituted finite "+field)}}
 noncanonical:=append(append([]byte{},raw...), '\n');file.Bytes=int64(len(noncanonical));file.SHA256=assemblyCatalogDigest(noncanonical);snap.State.SourceDescriptor.Digest,snap.State.SourceDescriptor.Bytes=file.SHA256,file.Bytes
 if checkDebianProducerSourceDescriptor(noncanonical,file,snap)==nil{t.Fatal("noncanonical source accepted with its own hash")}
}
func TestDebianProductNormativeRawRejectsHashShapedSelectedSubstitutions(t *testing.T){
 raw:=map[string][]byte{};for _,role:=range []string{"dependency-lock","acquisition-manifest","snapshot-source"}{data,e:=os.ReadFile(filepath.Join("testdata","document-assembly-debian","capture-v4",role+".fixture"));if e!=nil{t.Fatal(e)};raw[role]=data}
 if e:=checkDebianProductNormativeRaw(raw);e!=nil{t.Fatal(e)}
 for _,role:=range []string{"dependency-lock","acquisition-manifest","snapshot-source"}{bad:=map[string][]byte{};for key,value:=range raw{bad[key]=value};bad[role]=[]byte("synthetic selected bytes with a valid SHA shape");if checkDebianProductNormativeRaw(bad)==nil{t.Fatal("selected ref replaced normative original "+role)}}
}
func TestDebianProducerNativeClaimPersistsInBothViewsAndWorkerAdmission(t *testing.T){
 g:=executionTestGate(t,filepath.Join(t.TempDir(),"shared"));row:=nativeLeaseFixture();row.Scope=g.scope
 if e:=g.mutate(context.Background(),func(l *executionLeaseLedger)error{l.InputProofs=append(l.InputProofs,row);return nil});e!=nil{t.Fatal(e)}
 memory:=g.snapshot.Load();observed:=g.reservationObservation(context.Background())
 if memory==nil||memory.Count!=1||memory.MemoryMiB!=192||observed.Status!="ready"||observed.Count!=1||observed.MemoryMiB!=192{t.Fatal("native claim disappeared from mutable or readonly projection")}
 g.assemblyDiskProbe=func(p resourcebudget.DiskPath)(resourcebudget.Disk,error){return resourcebudget.Disk{Role:p.Role,ObservedAt:time.Now(),AvailableBytes:executionPointer(uint64(100)<<30)},nil}
 g.api.deps.ResourceSnapshot=func()resourcebudget.Snapshot{s:=executionFixtureSnapshot();s.Host.Memory.EffectiveAvailableBytes=executionPointer(uint64(3200)<<20);return s}
 actor:=management.ExecutionActorRef{Local:true,ProjectID:"native-pressure"}
 policy:=executionFixturePolicy();policy.MaxHeadroomMiB=2048
 decision,e:=g.reserve(context.Background(),"worker-pressure",actor.ProjectID,"api",2,actor,management.ExecutionAuthority{},policy)
 if e!=nil||decision.State!="waiting"{t.Fatalf("unknown native demand did not gate worker: %#v %v",decision,e)}
 ledger,e:=g.readReservationLedger(context.Background());if e!=nil||len(ledger.Leases)!=0||len(ledger.InputProofs)!=1{t.Fatal("waiting admission changed allocation ownership",e)}
 if e=g.advanceDebianInputProof(context.Background(),row.Identity,"finished",nil);e!=nil{t.Fatal(e)}
 decision,e=g.reserve(context.Background(),"worker-pressure",actor.ProjectID,"api",2,actor,management.ExecutionAuthority{},policy);if e!=nil||decision.State!="admitted"{t.Fatal("same capacity did not admit after exact native claim settlement",decision,e)}
}
func nativeLeaseFixture()debianInputProofExecutionLease{
 now:=time.Now().UTC();return debianInputProofExecutionLease{Scope:strings.Repeat("a",64),ID:"native-fixture",Actor:management.ExecutionActorRef{Local:true},Identity:runtimeinstall.DebianInputProofIdentity{JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"native-fixture",ProfileID:"p",ProfileSHA256:strings.Repeat("b",64),SourceSHA256:strings.Repeat("c",64),OwnerPID:123,OwnerCreationTime:456},Resources:backend.Resources{MemoryMiB:192,CPUMillis:1000},State:"unknown",DiskBytes:70<<20,CreatedAt:now,WholeDeadline:now.Add(60*time.Second)}
}
func TestDebianProducerUnknownLeaseDecoderViewsAndDemand(t *testing.T){
 row:=nativeLeaseFixture();ledger:=executionLeaseLedger{Version:1,Leases:[]executionLease{},InputProofs:[]debianInputProofExecutionLease{row}};data,_:=json.Marshal(ledger);decoded,e:=decodeExecutionLedger(data);if e!=nil||executionLeaseCount(decoded)!=1{t.Fatal(e)}
 view:=&executionReservationView{};addDebianInputProofReservationView(view,decoded.InputProofs);if view.Count!=1||view.MemoryMiB!=192||view.CPUMillis!=1000{t.Fatal("unknown demand disappeared")}
 d:=executionCapacityDemand{};if e=addDebianInputProofHostDemand(decoded.InputProofs,"","",&d);e!=nil||d.NativeHostMemoryBytes!=192<<20||d.NativeHostCPUMillis!=1000||d.AllMemory!=0||d.CPU!=0{t.Fatal("native claim was lost or fabricated guest demand")}
 ledger.InputProofs=append(ledger.InputProofs,row);data,_=json.Marshal(ledger);if _,e=decodeExecutionLedger(data);e==nil{t.Fatal("duplicate native identity accepted")}
}
func TestDebianProducerHostGateDoesNotInventDockerAndRequiresCommit(t *testing.T){
 now:=time.Now().UTC();total,available,commit:=uint64(16<<30),uint64(9<<30),uint64(10<<30);cpu,busy:=int64(4000),float64(10)
 s:=resourcebudget.Snapshot{Host:resourcebudget.HostSnapshot{Status:"ready",Platform:"windows",Scope:"control-plane-host",ObservedAt:now,Memory:resourcebudget.Memory{PhysicalAvailableBytes:&available,EffectiveTotalBytes:&total,CommitAvailableBytes:&commit},CPU:resourcebudget.CPU{EffectiveMillis:&cpu,UtilizationPercent:&busy}}}
 p:=product.SwarmSchedulerPolicy{MaxActive:4,MaxPerProject:2,HeadroomPercent:25,MinHeadroomMiB:512,MaxHeadroomMiB:1024,DiskReserveMiB:2048};d:=executionCapacityDemand{NativeHostMemoryBytes:192<<20,NativeHostCPUMillis:1000}
 if e:=evaluateDebianInputProofHostCapacity(s,p,d,now);e!=nil{t.Fatal(e)}
 commit=7<<30;if evaluateDebianInputProofHostCapacity(s,p,d,now)==nil{t.Fatal("commit refusal bypassed")};commit=10<<30;busy=50;if evaluateDebianInputProofHostCapacity(s,p,d,now)==nil{t.Fatal("CPU refusal bypassed")}
 if validateInputProofRetainedDaemonCapacity(s,p,d,now)==nil{t.Fatal("host observation fabricated missing daemon evidence")}
}
func TestDebianProducerActualAPINilWireIsPreReservationRefusal(t *testing.T){
 row:=nativeLeaseFixture();a:=&API{}
 out,e:=a.proveBundledDebianInputs(context.Background(),runtimeinstall.DebianInputProofRequest{Identity:row.Identity,Actor:row.Actor},func(runtimeinstall.DebianInputProofOutcome)error{return nil})
 if e==nil||out.ResourcesHeld||out.Phase!="blocked"{t.Fatal("nil Product/ledger wire gained native authority")};if _,ok:=liveDebianProducerRuns.Load(row.Identity.RunID);ok{t.Fatal("refused request registered a native owner")}
}
func TestDebianProducerCommandsAreClosedTenSerialStages(t *testing.T){
 p:=&debianProducerPlan{directory:filepath.Join(t.TempDir(),"owned"),paths:map[string]string{"gpg":"pinned-gpg","gpgv":"pinned-gpgv","cp-python":"pinned-python","projection-entry":"pinned-entry","projection-source":"pinned-projection"}};commands:=debianProducerCommands(p)
 if len(commands)!=10||commands[0][0]!="pinned-gpg"||commands[4][0]!="pinned-gpgv"||commands[9][0]!="pinned-python"||strings.Join(commands[9][1:4]," ")!="-I -S -B"{t.Fatal("fixed producer command graph changed")}
}
