package runtimeinstall

import("strings";"testing";"time";"crypto/sha256";"fmt";"bytes")

func TestConfiguredInputProofDescriptorRequiresIndependentExactBytes(t *testing.T){
 p:=BundledDebianInputProofProfile{ID:"fixture",Protocol:DebianInputProofProtocol,SourceSHA256:strings.Repeat("a",64),Files:[]DebianInputProofFile{{Role:"fixture",RelativePath:"fixture",Bytes:1,SHA256:strings.Repeat("b",64)}}}
 data,err:=CanonicalDebianInputProofProfile(p);if err!=nil{t.Fatal(err)};pin:=fmt.Sprintf("%x",sha256.Sum256(data))
 got,err:=DecodeDebianInputProofProfile(data,pin);if err!=nil||got.ID!=p.ID||got.SHA256!=pin||got.Root!=""{t.Fatal("exact descriptor did not decode",err)}
 for _,kind:=range []string{"wrong-pin","unknown-field","duplicate-key","noncanonical","unknown-purpose"}{t.Run(kind,func(t *testing.T){
  body:=append([]byte(nil),data...);expected:=pin
  switch kind{case "wrong-pin":expected=strings.Repeat("c",64);case "unknown-field":body=append(body[:len(body)-1],[]byte(`,"extra":1}`)...);case "duplicate-key":body=bytes.Replace(body,[]byte(`"schema":1`),[]byte(`"schema":1,"schema":1`),1);case "noncanonical":body=append(body,'\n');case "unknown-purpose":other:=p;other.Purpose="unknown";body,_=CanonicalDebianInputProofProfile(other)}
  if kind!="wrong-pin"{expected=fmt.Sprintf("%x",sha256.Sum256(body))};if _,err:=DecodeDebianInputProofProfile(body,expected);err==nil{t.Fatal("untrusted or malformed descriptor admitted")}
 })}
 if (DocumentRuntimeConfiguration{ProfileSHA256:pin}).Valid()||(DocumentRuntimeConfiguration{ToolDomainSHA256:pin}).Valid(){t.Fatal("partial configured trust accepted")}
}

func TestInputProofPurposeKeepsSeparateFiniteContracts(t *testing.T){
 legacy,ok:=DebianInputProofLimits("");if !ok||legacy.Children!=10||legacy.MemoryMiB!=192||legacy.Work!=40*time.Second||legacy.Whole!=60*time.Second{t.Fatal("legacy limits changed")}
 binary,ok:=DebianInputProofLimits(DebianBinaryInputProofPurpose);if !ok||binary.Children!=1||binary.MemoryMiB!=2048||binary.CPUMillis!=1000||binary.Work!=300*time.Second||binary.Whole!=420*time.Second{t.Fatal("binary arm not finite and separate")}
 if _,ok:=DebianInputProofLimits("binary");ok{t.Fatal("unknown purpose admitted")}
 id:=DebianInputProofIdentity{JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 other:=id;other.Purpose=DebianBinaryInputProofPurpose
 if !ValidInputProofIdentity(other)||SameInputProofOperation(id,other){t.Fatal("purpose was not captured in operation identity")}
 other.Purpose="unknown";if ValidInputProofIdentity(other){t.Fatal("unknown purpose accepted")}
}

func TestInputProofIdentityAllowsOnlyOnceKnownBinding(t *testing.T){
 id:=DebianInputProofIdentity{JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 if !ValidInputProofIdentity(id){t.Fatal("fixture invalid")};known:=id;known.OwnerPID=17;known.OwnerCreationTime=19;known.ExpectedCAS=InputProofCAS{Revision:1,Digest:strings.Repeat("c",64)}
 if !SameInputProofOperation(id,known)||SameInputProofOperation(known,id){t.Fatal("known identity changed/reverted")}
 bad:=known;bad.JobGeneration="other";if SameInputProofOperation(known,bad){t.Fatal("generation relabel")};bad=known;bad.RunID="../escape";if ValidInputProofIdentity(bad){t.Fatal("path identity allowed")}
}
func TestInputProofFactsCannotBecomeImageReadinessOrFalseSettlement(t *testing.T){
 id:=DebianInputProofIdentity{JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 out:=DebianInputProofOutcome{Identity:id,Phase:"unknown",ResourcesHeld:true};if !ValidInputProofOutcome(out){t.Fatal("unknown retained rejected")};out.CleanupConfirmed=true;if ValidInputProofOutcome(out){t.Fatal("held plus clean accepted")}
 out.ResourcesHeld=false;out.Phase="ready";if ValidInputProofOutcome(out){t.Fatal("proof became image ready")};out.Phase="facts-recorded";if ValidInputProofOutcome(out){t.Fatal("missing domains/head accepted")}
}

func TestBinaryProofOutcomeCannotBorrowSignatureDomainOrClaimRuntimeReady(t *testing.T){
 id:=DebianInputProofIdentity{Purpose:DebianBinaryInputProofPurpose,JobID:"j",JobGeneration:"g",RunID:"r",AllocationID:"a",ProfileID:"p",ProfileSHA256:strings.Repeat("a",64),SourceSHA256:strings.Repeat("b",64)}
 out:=DebianInputProofOutcome{Identity:id,Phase:"facts-recorded",CleanupConfirmed:true,PublishedCAS:InputProofCAS{Revision:2,Digest:strings.Repeat("c",64)}}
 for _,kind:=range []string{"signed-index-bindings","debian-binaries","input-closure","producer-raw-manifest"}{out.Domains=append(out.Domains,InputProofDomainRef{Kind:kind,SHA256:strings.Repeat("d",64),Bytes:128})}
 if !ValidInputProofOutcome(out){t.Fatal("exact binary fact publication refused")}
 for _,kind:=range []string{"signature-domain","legacy-purpose","ready","held","duplicate"}{t.Run(kind,func(t *testing.T){bad:=out;bad.Domains=append([]InputProofDomainRef(nil),out.Domains...);switch kind{case "signature-domain":bad.Domains[0].Kind="official-root-policy";case "legacy-purpose":bad.Identity.Purpose="";case "ready":bad.Phase="ready";case "held":bad.ResourcesHeld=true;case "duplicate":bad.Domains[0]=bad.Domains[1]};if ValidInputProofOutcome(bad){t.Fatal("different domain/settlement gained closure authority")}})}
}
