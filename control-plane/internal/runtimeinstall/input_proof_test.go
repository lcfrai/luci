package runtimeinstall

import("strings";"testing";"time")

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
