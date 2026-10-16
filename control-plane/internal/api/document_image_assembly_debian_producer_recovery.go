package api

import (
 "context"
 "errors"
 "github.com/lcfrai/luci/control-plane/internal/management"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Observe/retire only. Never runs a child, loads a profile, imports a key,
// adopts facts, calls image Reconcile or reconstructs an original capability.
func(a *API)observeBundledDebianInputProof(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,actor management.ExecutionActorRef,publish runtimeinstall.PublishDebianInputProof)(runtimeinstall.DebianInputProofOutcome,error){
 out:=runtimeinstall.DebianInputProofOutcome{Identity:id,Phase:"unknown",ResourcesHeld:true,ReasonCode:"input_proof_original_owner_unavailable",Message:"Original native ownership cannot be reconstructed after restart; the exact retained reservation remains held."}
 if publish==nil||a.execution==nil||!runtimeinstall.ValidInputProofIdentity(id){return out,runtimeinstall.ErrInputProofOwnership}
 if _,err:=a.authorizeRuntimeInstaller(ctx,actor);err!=nil{return out,err}
 ledger,err:=a.execution.readReservationLedger(ctx);if err!=nil{return out,err}
 found,finished:=false,false
 for _,row:=range ledger.InputProofs{if row.Scope==a.execution.scope&&row.ID==id.AllocationID{if row.Identity!=id||row.Actor!=actor{return out,runtimeinstall.ErrInputProofOwnership};found=true;finished=row.State=="finished";out.ResourcesHeld=!finished;out.CleanupConfirmed=finished}}
 value,live:=liveDebianProducerRuns.Load(id.RunID)
 if !live{if !found{return out,runtimeinstall.ErrInputProofOwnership};_ = publish(out);return out,errors.New("native original owner unavailable")}
 run:=value.(*debianProducerLiveRun);run.mu.Lock();same:=run.api==a&&run.actor==actor&&runtimeinstall.SameInputProofOperation(run.identity,id);done:=run.done;run.mu.Unlock()
 if !same{return out,runtimeinstall.ErrInputProofOwnership}
 run.retire()
 select{case<-done:run.mu.Lock();out,err=run.outcome,run.err;if finished&&run.resourceSettlementKnown{out.ResourcesHeld=false;out.CleanupConfirmed=true};run.mu.Unlock();if e:=publish(out);e!=nil{return out,e};return out,err
 case<-ctx.Done():return out,ctx.Err()}
}
