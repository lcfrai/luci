package api

import (
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/backend/dockerbe"
	"github.com/lcfrai/luci/control-plane/internal/documentinputs"
)

type assemblyDomainReaders struct {
	ctx context.Context
	owner backend.DocumentDomainInputs
	selected assemblyDomainSelection
	readers map[string]io.ReaderAt
}

func (r *assemblyDomainReaders) reader(input backend.DocumentImageAssemblyInput)(io.ReaderAt,error) {
	role:=r.selected.roles[input.Path]
	if role==""{return nil,errAssemblyClosureDomain}
	if retained:=r.readers[role];retained!=nil{return retained,nil}
	captured,err:=r.owner.ReaderAt(r.ctx,role);if err!=nil{return nil,err}
	r.readers[role]=captured
	return captured,nil
}

func (r *assemblyDomainReaders) bytes(input backend.DocumentImageAssemblyInput,limit int64)([]byte,error) {
	if input.Bytes<1||input.Bytes>limit{return nil,errAssemblyClosureDomain}
	reader,err:=r.reader(input);if err!=nil{return nil,err}
	data:=make([]byte,int(input.Bytes));n,err:=reader.ReadAt(data,0)
	if n!=len(data)||err!=nil&&err!=io.EOF||assemblyCatalogDigest(data)!=input.Digest{return nil,errAssemblyClosureDomain}
	return data,nil
}

func assemblyInspectionPin(input backend.DocumentImageAssemblyInput)documentinputs.Pin {
	return documentinputs.Pin{SHA256:input.Digest,Bytes:input.Bytes}
}

func assemblyDependencyObservation(p documentinputs.Package)assemblyDependencyPackage {
	join:=func(v []string)string{return strings.Join(strings.Fields(strings.Join(v,", ")), " ")}
	return assemblyDependencyPackage{Name:p.Name,Version:p.Version,Architecture:p.Architecture,MultiArch:p.MultiArch,
		Depends:join(p.Depends),PreDepends:join(p.PreDepends),Provides:join(p.Provides),Breaks:join(p.Breaks),Conflicts:join(p.Conflicts),
		Source:p.Source,BuiltUsing:join(p.BuiltUsing),StaticBuiltUsing:join(p.StaticBuiltUsing)}
}

// Only this constructor joins actual held originals, the positively completed
// binary reader, authenticated index bindings and real archive observations.
// Selected metadata supplies expectations; it cannot supply observed facts.
func captureDocumentAssemblyDomains(ctx context.Context,a *API,p *debianProducerPlan,binary *debianBinarySemanticCapture)(*assemblyObservedDomains,error) {
	if ctx==nil||ctx.Err()!=nil||a==nil||a.execution==nil||p==nil||p.binary==nil||binary==nil||binary.plan!=p||
		binary.spool==nil||binary.spool.owner==nil||!binary.spool.owner.positiveBinaryCompletion(binary.child)||!binary.spool.current(ctx){return nil,errAssemblyClosureDomain}
	if err:=p.current(ctx,a);err!=nil{return nil,err}
	selected,err:=readAssemblyDomainSelection(ctx,p);if err!=nil{return nil,err}
	demand,err:=dockerbe.PlanDocumentDomainInputs(p.profile.Root,selected.objects);if err!=nil{return nil,err}
	if err=a.execution.reserveDocumentDomainInputs(ctx,p.identity,demand);err!=nil{return nil,err}
	p.domainMu.Lock()
	if p.domainInputs!=nil||p.domainOpening||p.domainRetiring{p.domainMu.Unlock();return nil,errAssemblyClosureDomain}
	p.domainOpening=true;p.domainMu.Unlock()
	owner,openErr:=dockerbe.OpenDocumentDomainInputs(ctx,p.profile.Root,selected.objects)
	p.domainMu.Lock();p.domainInputs=owner;p.domainOpening=false;retiring:=p.domainRetiring;p.domainMu.Unlock()
	// The live plan owns every partial return. It alone can release the journal
	// charge after a positive Close, including errors before a candidate exists.
	if openErr!=nil{return nil,openErr}
	if owner==nil||retiring||owner.Demand()!=demand{return nil,errAssemblyClosureDomain}
	readers:=assemblyDomainReaders{ctx:ctx,owner:owner,selected:selected,readers:map[string]io.ReaderAt{}}
	out:=&assemblyObservedDomains{plan:p,binary:binary,domains:owner,helper:selected.helper.Helper,
		trust:assemblyObservedTrust{Policy:p.binary.policy,Keys:p.binary.keys,Signatures:p.binary.signatures,Pins:p.binary.pins},
		guards:[]backend.DocumentImageAssemblyEvidenceGuard{p.inputs,p.binary.indexes,p.binary.sourceIndexes,p.binary.small,p.binary.selectionGuard}}
	baseReader,err:=readers.reader(selected.base.Layer);if err!=nil{return nil,err}
	base,err:=documentinputs.InspectBase(ctx,io.NewSectionReader(baseReader,0,selected.base.Layer.Bytes),assemblyInspectionPin(selected.base.Layer));if err!=nil{return nil,err}
	out.base.Layer=selected.base.Layer
	universe:=map[string]assemblyDependencyPackage{}
	for _,pkg:=range base.Packages {
		value:=assemblyDependencyObservation(pkg)
		if _,exists:=universe[value.Name];exists{return nil,errAssemblyClosureDomain}
		universe[value.Name]=value;out.base.Packages=append(out.base.Packages,value)
	}
	inventory:=newAssemblyInputInventory()
	if err=inventory.addBase(base);err!=nil{return nil,err}
	// Decode the same exact package tuples admitted and read by the original
	// binary process; source control relationships are compared below as well.
	for _,tuple:=range p.binary.selection.Packages {
		role:=fmt.Sprintf("package.%03d",tuple.Ordinal)
		var input backend.DocumentImageAssemblyInput
		for _,item:=range selected.paths.Files{if item.Role==role{input=backend.DocumentImageAssemblyInput{Path:item.RelativePath,Bytes:item.Bytes,Digest:item.SHA256};break}}
		if input.Bytes!=tuple.Bytes||input.Digest!=tuple.SHA256{return nil,errAssemblyClosureDomain}
		reader,e:=readers.reader(input);if e!=nil{return nil,e}
		actual,e:=documentinputs.InspectDeb(ctx,reader,input.Bytes,assemblyInspectionPin(input),documentinputs.DebExpectation{Name:tuple.Name,Version:tuple.Version,Architecture:tuple.Architecture})
		if e!=nil{return nil,e}
		value:=assemblyDependencyObservation(actual.Package)
		if e=compareAssemblyBinaryControl(binary,tuple.Ordinal,value);e!=nil{return nil,e}
		universe[value.Name]=value
		if e=inventory.addDeb(actual);e!=nil{return nil,e}
	}
	for _,wheel:=range selected.wheels.Wheels {
		reader,e:=readers.reader(wheel.Archive);if e!=nil{return nil,e}
		actual,e:=documentinputs.InspectWheel(ctx,reader,wheel.Archive.Bytes,assemblyInspectionPin(wheel.Archive),
			documentinputs.WheelOwner{ID:wheel.Distribution,Version:wheel.Version,InstalledOwner:wheel.InstalledOwner})
		if e!=nil{return nil,e}
		if wheel.ID!="wheel/"+strings.ToLower(strings.ReplaceAll(wheel.Distribution,"_","-")){return nil,errAssemblyClosureDomain}
		components,e:=inventory.addWheel(wheel,actual);if e!=nil{return nil,e}
		entries:=make([]assemblyObservedWheelEntryPoint,0,len(actual.EntryPoints))
		for _,entry:=range actual.EntryPoints {
			entries=append(entries,assemblyObservedWheelEntryPoint{Group:entry.Group,Name:entry.Name,Value:entry.Value})
		}
		out.wheels=append(out.wheels,assemblyObservedWheel{ID:wheel.ID,Version:wheel.Version,Archive:wheel.Archive,Components:components,
			RequiresDist:append([]string(nil),actual.RequiresDist...),RequiresPython:actual.RequiresPython,
			Tags:append([]string(nil),actual.Tags...),RootIsPurelib:actual.RootIsPurelib,
			InstalledOwner:actual.InstalledOwner,EntryPoints:entries})
		out.sources=append(out.sources,assemblyObservedSource{ID:wheel.ID,Version:wheel.Version,Archives:append([]backend.DocumentImageAssemblyInput(nil),wheel.Sources...)})
	}
	if err=captureAssemblySourceArchives(&readers,p.binary,out);err!=nil{return nil,err}
	out.sources=append(out.sources,assemblyObservedSource{ID:"luci",Version:out.helper.SourceCommit,Archives:append([]backend.DocumentImageAssemblyInput(nil),selected.helper.Sources...)})
	if err=inventory.finish(universe);err!=nil{return nil,err}
	out.installed=inventory.sorted()
	if inventory.hasBaseOwner {
		out.sources=append(out.sources,assemblyObservedSource{ID:"base/rootfs",Version:selected.base.Layer.Digest,Archives:[]backend.DocumentImageAssemblyInput{selected.base.Layer}})
	}
	for _,notice:=range selected.notices.Notices {
		raw,e:=readers.bytes(notice.File,1<<20);if e!=nil{return nil,e}
		if notice.Component!="luci"&&!inventory.noticeMatches(notice.Component,assemblyCatalogDigest(raw),int64(len(raw))){return nil,errAssemblyClosureDomain}
		out.notices=append(out.notices,notice)
	}
	// Hashing the source-retention original is a binding to the configured
	// source statement. It is not an assertion that declared licensing terms
	// were adjudicated by an archive parser.
	retention,e:=readers.bytes(selected.notices.Retention,1<<20)
	if e!=nil||assemblyCatalogDigest(retention)!=out.helper.SourceRetentionDigest{return nil,errAssemblyClosureDomain}
	if out.helper.RecipeDigest!=backend.DocumentImageAssemblyRecipeDigest(){return nil,errAssemblyClosureDomain}
	for _,input:=range []backend.DocumentImageAssemblyInput{out.helper.Extractor,out.helper.Verifier,out.helper.Requirements,out.helper.DependencyLock} {
		if _,e=readers.bytes(input,2<<20);e!=nil{return nil,e}
	}
	sort.Slice(out.sources,func(i,j int)bool{return out.sources[i].ID+"@"+out.sources[i].Version<out.sources[j].ID+"@"+out.sources[j].Version})
	if err=out.current(ctx,a);err!=nil{return nil,err}
	return out,nil
}

func compareAssemblyBinaryControl(capture *debianBinarySemanticCapture,ordinal int,pkg assemblyDependencyPackage)error {
	if capture==nil||ordinal<0||ordinal>=174{return errAssemblyClosureDomain}
	// Captured during the original successful kind-3 semantic pass. This does
	// not reopen or reread a raw bundle under an already spent read budget.
	actual:=capture.controls[ordinal]
	for _,field:=range []struct{observed *string;expected string;relation bool}{
		{actual.Package,pkg.Name,false},{actual.Version,pkg.Version,false},{actual.Architecture,pkg.Architecture,false},{actual.MultiArch,pkg.MultiArch,false},
		{actual.Depends,pkg.Depends,true},{actual.PreDepends,pkg.PreDepends,true},{actual.Provides,pkg.Provides,true},{actual.Breaks,pkg.Breaks,true},
		{actual.Conflicts,pkg.Conflicts,true},{actual.Source,pkg.Source,false},{actual.BuiltUsing,pkg.BuiltUsing,true},{actual.StaticBuiltUsing,pkg.StaticBuiltUsing,true},
	} {
		value:="";if field.observed!=nil{value=*field.observed}
		normalize:=func(raw string)string{return strings.Join(strings.Fields(raw)," ")}
		if field.relation {
			normalize=func(raw string)string {
				groups:=strings.Split(raw,",")
				for i,group:=range groups {
					alternatives:=strings.Split(group,"|")
					for j,value:=range alternatives{alternatives[j]=strings.Join(strings.Fields(value)," ")}
					groups[i]=strings.Join(alternatives,"|")
				}
				return strings.Join(groups,",")
			}
		}
		if normalize(value)!=normalize(field.expected){return errAssemblyClosureDomain}
	}
	return nil
}

// Helper paths inside wheels use Debian's fixed Python 3.11 local scheme.
func assemblyWheelInstalledPath(name string)(string,error) {
	if !assemblyClosureSafePath(name){return "",errAssemblyClosureDomain}
	parts:=strings.Split(name,"/")
	if strings.HasSuffix(parts[0],".data") {
		if len(parts)<3{return "",errAssemblyClosureDomain}
		switch parts[1] {
		case "purelib","platlib":return path.Join("usr/local/lib/python3.11/dist-packages",path.Join(parts[2:]...)),nil
		case "data":return path.Join("usr/local",path.Join(parts[2:]...)),nil
		default:return "",errAssemblyClosureDomain // scripts/headers need an explicit recipe
		}
	}
	return path.Join("usr/local/lib/python3.11/dist-packages",name),nil
}
