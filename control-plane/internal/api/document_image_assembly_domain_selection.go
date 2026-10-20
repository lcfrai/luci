package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These documents select exact inputs under the application-pinned profile.
// They are not observations, attestations or permission to assemble an image.
type assemblyBaseSelection struct {
	Schema int `json:"schema"`
	Layer backend.DocumentImageAssemblyInput `json:"layer"`
}

type assemblyWheelSelection struct {
	ID string `json:"id"`
	Version string `json:"version"`
	Distribution string `json:"distribution"`
	InstalledOwner string `json:"installedOwner"`
	Archive backend.DocumentImageAssemblyInput `json:"archive"`
	Sources []backend.DocumentImageAssemblyInput `json:"sources"`
	Native []assemblyObservedNativeComponent `json:"native"`
}

type assemblyWheelsSelection struct {
	Schema int `json:"schema"`
	Wheels []assemblyWheelSelection `json:"wheels"`
}

type assemblySourcesSelection struct {
	Schema int `json:"schema"`
	Sources []assemblySourceSelected `json:"sources"`
	Indexes []assemblySourceIndexSelection `json:"indexes"`
}

type assemblySourceSelected struct {
	assemblyObservedSource
	ArchiveNames []string `json:"archiveNames"` // signed pool paths, in archive order
}

type assemblySourceIndexSelection struct {
	ReleaseOrdinal int `json:"releaseOrdinal"`
	CandidateID string `json:"candidateId"`
	SHA256 string `json:"sha256"`
	CleartextSHA256 string `json:"cleartextSha256"`
	CleartextBytes int64 `json:"cleartextBytes"`
	Sources []string `json:"sources"` // exact qualified source ID@version
}

type assemblyNoticesSelection struct {
	Schema int `json:"schema"`
	Retention backend.DocumentImageAssemblyInput `json:"retention"`
	Notices []assemblyObservedNotice `json:"notices"`
}

type assemblyHelperSelection struct {
	Schema int `json:"schema"`
	Helper assemblyObservedHelper `json:"helper"`
	Sources []backend.DocumentImageAssemblyInput `json:"sources"`
}

type assemblyAlgorithmSelection struct {
	Schema int `json:"schema"`
	Algorithm string `json:"algorithm"`
}

type assemblyDomainSelection struct {
	base assemblyBaseSelection
	wheels assemblyWheelsSelection
	sources assemblySourcesSelection
	notices assemblyNoticesSelection
	helper assemblyHelperSelection
	paths debianBinaryInputPaths
	objects []backend.DomainInput
	roles map[string]string
}

func readAssemblyDomainSelection(ctx context.Context,p *debianProducerPlan)(assemblyDomainSelection,error) {
	var selected assemblyDomainSelection
	if ctx==nil||ctx.Err()!=nil||p==nil||p.binary==nil||p.inputs==nil {return selected,errAssemblyClosureDomain}
	var dependency,inventory assemblyAlgorithmSelection
	for _,entry:=range []struct{role string;target any}{
		{assemblyBaseRootfsRole,&selected.base},{assemblyWheelsRole,&selected.wheels},
		{assemblyDebianSourcesRole,&selected.sources},{assemblyNoticesSourcesRole,&selected.notices},
		{assemblyLuciHelperRole,&selected.helper},{assemblyDependencyClosureRole,&dependency},
		{assemblyInstalledFilesRole,&inventory},{"binary-input-paths",&selected.paths},
	} {
		data,err:=readDebianPlanInput(ctx,p,entry.role)
		if err!=nil{return selected,err}
		decoder:=json.NewDecoder(bytes.NewReader(data));decoder.DisallowUnknownFields()
		if decoder.Decode(entry.target)!=nil{return selected,errAssemblyClosureDomain}
		canonical,err:=json.Marshal(entry.target)
		if err!=nil||!bytes.Equal(canonical,data){return selected,errAssemblyClosureDomain}
	}
	if selected.base.Schema!=1||selected.wheels.Schema!=1||selected.sources.Schema!=1||selected.notices.Schema!=1||selected.helper.Schema!=1||selected.paths.Schema!=1||
		dependency.Schema!=1||dependency.Algorithm!="debian-binary-relations-v1"||
		inventory.Schema!=1||inventory.Algorithm!="base-debian-wheel-input-inventory-v1"||
		len(selected.wheels.Wheels)!=2||len(selected.sources.Sources)==0||len(selected.sources.Sources)>512||
		len(selected.notices.Notices)==0||len(selected.notices.Notices)>512||
		len(selected.sources.Indexes)==0||len(selected.sources.Indexes)>12||
		len(selected.helper.Sources)==0||len(selected.helper.Sources)>8 {return selected,errAssemblyClosureDomain}

	// The native binary selection already supplies independently bound package
	// and signed index tuples. This second reader may not nominate different
	// bytes under a matching package name.
	if _,_,err:=debianBinaryDeclaredObjects(mustMarshalAssemblyPaths(selected.paths),p.profile.Root,p.binary);err!=nil{return selected,err}
	selected.roles=map[string]string{}
	seen:=map[string]backend.DomainInput{}
	add:=func(role string,input backend.DocumentImageAssemblyInput)error{
		if !assemblyClosureSafePath(input.Path)||input.Bytes<=0||input.Bytes>backend.DocumentDomainInputFileBytes||!assemblyCatalogHex(input.Digest)||
			strings.ContainsAny(input.Path,":\\")||path.Clean(input.Path)!=input.Path{return errAssemblyClosureDomain}
		item:=backend.DomainInput{Role:role,RelativePath:input.Path,Bytes:input.Bytes,SHA256:input.Digest}
		fold:=strings.ToLower(input.Path)
		if prior,ok:=seen[fold];ok {
			if prior.RelativePath!=item.RelativePath||prior.Bytes!=item.Bytes||prior.SHA256!=item.SHA256{return errAssemblyClosureDomain}
			selected.roles[input.Path]=prior.Role
			return nil
		}
		seen[fold]=item;selected.roles[input.Path]=role;selected.objects=append(selected.objects,item)
		return nil
	}
	if selected.base.Layer.Bytes>128<<20{return selected,errAssemblyClosureDomain}
	if err:=add("base-layer",selected.base.Layer);err!=nil{return selected,err}
	for _,entry:=range selected.paths.Files {
		if err:=add(entry.Role,backend.DocumentImageAssemblyInput{Path:entry.RelativePath,Bytes:entry.Bytes,Digest:entry.SHA256});err!=nil{return selected,err}
	}
	for i,wheel:=range selected.wheels.Wheels {
		if !strings.HasPrefix(wheel.ID,"wheel/")||len(wheel.Sources)==0||len(wheel.Sources)>8||wheel.Archive.Bytes>128<<20{return selected,errAssemblyClosureDomain}
		if err:=add(fmt.Sprintf("wheel.%d",i),wheel.Archive);err!=nil{return selected,err}
		for n,input:=range wheel.Sources {if err:=add(fmt.Sprintf("wheel-source.%d.%d",i,n),input);err!=nil{return selected,err}}
	}
	for i,source:=range selected.sources.Sources {
		if !assemblyClosureComponent(source.ID)||!strings.HasPrefix(source.ID,"debian/")&&!strings.HasPrefix(source.ID,"wheel/")||
			len(source.Archives)==0||len(source.Archives)>8||strings.HasPrefix(source.ID,"debian/")&&len(source.ArchiveNames)!=len(source.Archives)||
			strings.HasPrefix(source.ID,"wheel/")&&len(source.ArchiveNames)!=0{return selected,errAssemblyClosureDomain}
		for n,input:=range source.Archives {if err:=add(fmt.Sprintf("debian-source.%d.%d",i,n),input);err!=nil{return selected,err}}
	}
	for i,notice:=range selected.notices.Notices {if notice.File.Bytes>1<<20{return selected,errAssemblyClosureDomain};if err:=add(fmt.Sprintf("notice.%d",i),notice.File);err!=nil{return selected,err}}
	for i,input:=range selected.helper.Sources {if err:=add(fmt.Sprintf("luci-source.%d",i),input);err!=nil{return selected,err}}
	executable,ok:=p.files["assembly-helper-executable"]
	if !ok||executable.SHA256!=selected.helper.Helper.HelperDigest{return selected,errAssemblyClosureDomain}
	if err:=add("assembly-helper-executable",backend.DocumentImageAssemblyInput{Path:executable.RelativePath,Bytes:executable.Bytes,Digest:executable.SHA256});err!=nil{return selected,err}
	for _,entry:=range []struct{role string;input backend.DocumentImageAssemblyInput}{
		{"source-retention",selected.notices.Retention},{"extractor",selected.helper.Helper.Extractor},
		{"verifier",selected.helper.Helper.Verifier},{"requirements",selected.helper.Helper.Requirements},
		{"dependency-lock",selected.helper.Helper.DependencyLock},
	} {if err:=add(entry.role,entry.input);err!=nil{return selected,err}}
	if len(selected.objects)>512{return selected,errAssemblyClosureDomain}
	sort.Slice(selected.objects,func(i,j int)bool{return selected.objects[i].Role<selected.objects[j].Role})
	if ctx.Err()!=nil{return selected,ctx.Err()}
	return selected,nil
}

func mustMarshalAssemblyPaths(value debianBinaryInputPaths)[]byte {
	// The closed struct contains no values for which encoding/json can fail.
	data,_:=json.Marshal(value)
	return data
}

func(p *debianProducerPlan)retireDocumentDomains()bool {
	if p==nil{return true}
	p.domainMu.Lock();p.domainRetiring=true;owner:=p.domainInputs;opening:=p.domainOpening;p.domainMu.Unlock()
	if opening{return false}
	if owner==nil{return true}
	if err:=owner.Close();err!=nil{return false}
	p.domainMu.Lock()
	if p.domainInputs==owner{p.domainInputs=nil}
	p.domainMu.Unlock()
	return true
}

func documentAssemblyDomainInputRecords(ctx context.Context,a *API,p *debianProducerPlan)([]backend.DomainInput,error) {
	if p==nil||a==nil{return nil,errAssemblyClosureDomain}
	if err:=p.current(ctx,a);err!=nil{return nil,err}
	selected,err:=readAssemblyDomainSelection(ctx,p)
	if err!=nil{return nil,err}
	return append([]backend.DomainInput(nil),selected.objects...),nil
}
