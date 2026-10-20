package dockerbe

import (
 "context"
 "testing"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestBinarySelectionLoanRequiresSeparateConcreteOriginalOwners(t *testing.T){
 // No native source operation: all shapes are refused at the typed metadata
 // boundary, before package/evidence loan quota or native lookup can occur.
 for _,selection:=range []backend.DocumentImageAssemblyEvidenceGuard{nil,&assemblyEvidenceToken{}}{
  if got,e:=ReserveDocumentImageAssemblyBinaryReadLoanWithSelection(context.Background(),backend.DocumentImageAssemblyBinaryReadRequest{},nil,nil,nil,selection);got!=nil||e==nil{t.Fatal("foreign/unprepared selection admitted",got,e)}
 }
}

func TestBinarySelectionReservationReportsAllConservativeSourceCharges(t *testing.T){
 r:=&DocumentImageAssemblyBinaryReadReservation{sourceCharged:true,selectionCharged:true,sourceActive:true,selectionActive:true}
 got:=r.State()
 if got.SourceBytesCharged!=2*backend.DocumentImageAssemblyBinarySourceLoanBytes||got.SourceCallsCharged!=2*backend.DocumentImageAssemblyBinarySourceLoanCalls||got.ActiveGuardBorrows!=2{t.Fatal(got)}
 // Release must retain an active late selection even if other groups settled.
 r.sources=&assemblyEvidenceToken{owner:&assemblyEvidenceOwner{generation:1},generation:1};r.sourceActive=false
 r.selection=&assemblyEvidenceToken{owner:&assemblyEvidenceOwner{generation:1},generation:1}
 r.selection.owner.loan=r;r.selection.owner.mu.Lock()
 if e:=r.ReleaseUnallocated();e==nil||r.released||!r.selectionActive{t.Fatal("busy selection borrow discarded",e)}
 r.selection.owner.mu.Unlock()
 if e:=r.ReleaseUnallocated();e!=nil||!r.released||r.selectionActive{t.Fatal("actual selection return not settled",e)}
}

func TestBinarySelectionLoanKeepsSnapshottedSourcesAndLateSelectionSeparate(t *testing.T){
 for _,kind:=range []string{"admitted","selection-quota-refused","selection-owner-reused"}{t.Run(kind,func(t *testing.T){
  request,packages,indexes,sources:=assemblyBinaryReservationFixture()
  object:=sources.owner.objects[request.SmallSources[0].Role];delete(sources.owner.objects,object.Role)
  selection:=&assemblyEvidenceToken{owner:&assemblyEvidenceOwner{guard:&assemblySourceGuard{},ready:true,generation:1,objects:map[string]backend.DocumentImageAssemblyEvidenceObject{object.Role:object}},generation:1}
  if kind=="selection-quota-refused"{selection.owner.readBytes=backend.DocumentImageAssemblyEvidenceMaxBytes}
  if kind=="selection-owner-reused"{selection=sources}
  r,e:=ReserveDocumentImageAssemblyBinaryReadLoanWithSelection(context.Background(),request,packages,indexes,sources,selection)
  if kind=="selection-owner-reused"{if e==nil||r!=nil||packages.owner.loan!=nil{t.Fatal("same collector substituted selection",r,e)};return}
  if r==nil{t.Fatal("partial originals lost",e)}
  if kind=="admitted"{
   if e!=nil||r.State().ActiveGuardBorrows!=4||r.sources!=sources||r.selection!=selection||len(sources.owner.objects)!=2||len(selection.owner.objects)!=1{t.Fatal("original custody replaced",r.State(),e)}
  }else{if e==nil||r.State().ActiveGuardBorrows!=3||r.selectionActive||sources.owner.loan!=r||packages.owner.loan!=r{t.Fatal("failed final charge discarded earlier borrows",e)}}
  sourceBytes,selectedBytes:=sources.owner.readBytes,selection.owner.readBytes
  if e=r.ReleaseUnallocated();e!=nil||!r.released||packages.owner.loan!=nil||sources.owner.loan!=nil||selection.owner.loan!=nil{t.Fatal("exact metadata retirement failed",e)}
  if sources.owner.readBytes!=sourceBytes||selection.owner.readBytes!=selectedBytes{t.Fatal("retirement refunded quota")}
 })}
}
