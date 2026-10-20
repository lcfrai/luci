package api

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// This is an additional, finite original-input census within the already
// reserved binary proof's 2 GiB host operation. It neither reuses native binary
// handle slots nor changes that operation's memory, CPU or absolute deadline.
func validDocumentDomainDemand(d backend.DocumentDomainInputDemand) bool {
	return d.Inputs>0&&d.Inputs<=backend.DocumentDomainInputLimit&&
		d.OriginalHandles>d.Inputs&&d.OriginalHandles<=backend.DocumentDomainInputHandleLimit&&
		d.DeclaredBytes>0&&d.DeclaredBytes<=backend.DocumentDomainInputTotalBytes&&
		d.ReadCalls==backend.DocumentDomainInputReadCalls&&d.ReadBytes==backend.DocumentDomainInputReadBytes&&
		d.BlockBytes==backend.DocumentDomainInputBlockBytes&&d.ReadBufferBytes==2*backend.DocumentDomainInputBlockBytes&&
		d.AcquisitionWorkers==1&&d.RetirementWorkers==1&&d.CancellationWorkers==1&&
		d.CancellationCallbacks==d.Inputs+2
}

func (g *executionAdmission) reserveDocumentDomainInputs(ctx context.Context,id runtimeinstall.DebianInputProofIdentity,d backend.DocumentDomainInputDemand)error {
	if ctx==nil||ctx.Err()!=nil||g==nil||g.api==nil||id.Purpose!=runtimeinstall.DebianBinaryInputProofPurpose||!validDocumentDomainDemand(d) {
		return runtimeinstall.ErrInputProofOwnership
	}
	return g.mutate(ctx,func(l *executionLeaseLedger)error{
		if err:=g.api.deps.Product.CheckRuntimeInputProofCurrent(ctx,id);err!=nil{return err}
		for i:=range l.InputProofs {
			r:=&l.InputProofs[i]
			if r.Scope!=g.scope||r.ID!=id.AllocationID{continue}
			// Single use: an uncertain journal write or a partial open can never
			// be converted into another acquisition under the same lease.
			if r.Identity!=id||r.DomainInputs!=nil||r.State=="finished"||r.State=="unknown"||r.State=="retiring"||
				!validDebianInputProofLease(*r)||!time.Now().Before(r.WholeDeadline){return runtimeinstall.ErrInputProofOwnership}
			deadline,ok:=ctx.Deadline()
			if !ok||deadline.After(r.WholeDeadline){return runtimeinstall.ErrInputProofOwnership}
			policy,err:=g.api.deps.Product.SwarmSchedulerPolicy();if err!=nil{return err}
			demand:=executionCapacityDemand{}
			if err=addDebianInputProofHostDemand(l.InputProofs,"","",&demand);err!=nil{return err}
			// The fixed charge remains present until original cleanup completes;
			// a child process exiting does not release this later inspection.
			if err=evaluateDebianInputProofHostCapacity(g.api.deps.ResourceSnapshot(),policy,demand,time.Now().UTC());err!=nil{return err}
			copy:=d;r.DomainInputs=&copy
			return nil
		}
		return runtimeinstall.ErrInputProofOwnership
	})
}
