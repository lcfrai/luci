package runtimeinstall

import("strings";"testing")

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
