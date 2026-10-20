package api

import (
	"io"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/documentinputs"
)

func captureAssemblySourceArchives(r *assemblyDomainReaders,b *debianBinaryProducerInputs,out *assemblyObservedDomains)error {
	if r==nil||b==nil||out==nil{return errAssemblyClosureDomain}
	selected:=r.selected.sources
	sources:=map[string]assemblySourceSelected{}
	for _,source:=range selected.Sources {
		key:=source.ID+"@"+source.Version
		if _,exists:=sources[key];exists{return errAssemblyClosureDomain}
		sources[key]=source
	}
	covered:=map[string]bool{}
	physical:=map[string]bool{}
	for _,index:=range selected.Indexes {
		if r.ctx.Err()!=nil{return r.ctx.Err()}
		if index.ReleaseOrdinal<0||index.ReleaseOrdinal>=len(b.signatures.Releases)||
			len(index.Sources)==0||len(index.Sources)>256||!assemblyCatalogHex(index.SHA256)||
			!assemblyCatalogHex(index.CleartextSHA256)||index.CleartextBytes<1||index.CleartextBytes>128<<20{return errAssemblyClosureDomain}
		var bound *assemblyDebianIndexBindingV1
		for i:=range b.indexRecord.Bindings {
			value:=&b.indexRecord.Bindings[i]
			if value.IndexPath=="main/source/Sources.xz"&&value.CandidateID==index.CandidateID&&value.ReleaseOrdinal==index.ReleaseOrdinal&&value.CompressedObject.SHA256==index.SHA256 {
				if bound!=nil{return errAssemblyClosureDomain};bound=value
			}
		}
		if bound==nil||physical[index.SHA256]{return errAssemblyClosureDomain}
		physical[index.SHA256]=true
		release:=b.signatures.Releases[index.ReleaseOrdinal]
		logicalFound:=false
		for _,logical:=range release.Logical {
			if logical.CandidateID!=index.CandidateID{continue}
			for _,claim:=range logical.SignedIndexes {
				if claim.Path==bound.IndexPath&&claim.Bytes==bound.CompressedObject.Bytes&&claim.SHA256==bound.CompressedObject.SHA256{logicalFound=true}
			}
		}
		if !logicalFound{return errAssemblyClosureDomain}
		var compressed backend.DocumentImageAssemblyInput
		for _,file:=range r.selected.paths.Files {
			if file.Role=="xz-index:"+index.SHA256{compressed=backend.DocumentImageAssemblyInput{Path:file.RelativePath,Bytes:file.Bytes,Digest:file.SHA256};break}
		}
		if compressed.Bytes!=bound.CompressedObject.Bytes||compressed.Digest!=bound.CompressedObject.SHA256{return errAssemblyClosureDomain}
		reader,err:=r.reader(compressed);if err!=nil{return err}
		inputs:=[]documentinputs.SourceArchiveInput{}
		expected:=map[string]assemblySourceSelected{}
		for _,key:=range index.Sources {
			source,ok:=sources[key]
			if !ok||covered[key]||!strings.HasPrefix(source.ID,"debian/"){return errAssemblyClosureDomain}
			covered[key]=true;expected[key]=source
			for n,archive:=range source.Archives {
				original,e:=r.reader(archive);if e!=nil{return e}
				inputs=append(inputs,documentinputs.SourceArchiveInput{
					SourceName:strings.TrimPrefix(source.ID,"debian/"),SourceVersion:source.Version,
					FileName:source.ArchiveNames[n],Pin:assemblyInspectionPin(archive),Reader:original,
				})
			}
		}
		observed,err:=documentinputs.InspectSources(r.ctx,io.NewSectionReader(reader,0,compressed.Bytes),documentinputs.SourcesBinding{
			SignedReleaseSHA256:release.SignedRelease.SHA256,IndexPath:bound.IndexPath,CompressedIndex:assemblyInspectionPin(compressed),
			Cleartext:documentinputs.Pin{SHA256:index.CleartextSHA256,Bytes:index.CleartextBytes},
		},inputs)
		if err!=nil||len(observed)!=len(expected){return errAssemblyClosureDomain}
		for _,actual:=range observed {
			key:="debian/"+actual.Name+"@"+actual.Version
			source,ok:=expected[key]
			if !ok||len(actual.Archives)!=len(source.Archives){return errAssemblyClosureDomain}
			actualByName:=map[string]documentinputs.SourceArchive{}
			for _,archive:=range actual.Archives{if _,exists:=actualByName[archive.FileName];exists{return errAssemblyClosureDomain};actualByName[archive.FileName]=archive}
			for n,input:=range source.Archives {
				archive,ok:=actualByName[source.ArchiveNames[n]]
				if !ok||archive.Bytes!=input.Bytes||archive.SHA256!=input.Digest{return errAssemblyClosureDomain}
			}
			out.sources=append(out.sources,source.assemblyObservedSource)
			delete(expected,key)
		}
		if len(expected)!=0{return errAssemblyClosureDomain}
	}
	for key,source:=range sources {
		if strings.HasPrefix(source.ID,"debian/") {
			if !covered[key]{return errAssemblyClosureDomain}
			continue
		}
		// Wheel upstream archives have no Debian signature authority. Their
		// exact release/source relationship is the independently pinned profile
		// statement; original bytes were hashed during this owner's acquisition.
		if !strings.HasPrefix(source.ID,"wheel/")||covered[key]{return errAssemblyClosureDomain}
		if err:=appendAssemblyObservedSource(out,source.assemblyObservedSource);err!=nil{return err}
	}
	return nil
}

// A wheel's primary source may also be selected in the common source table.
// Collapse only an exactly identical selection; conflicting originals refuse.
func appendAssemblyObservedSource(out *assemblyObservedDomains,source assemblyObservedSource)error {
	if out==nil{return errAssemblyClosureDomain}
	for _,existing:=range out.sources {
		if existing.ID!=source.ID||existing.Version!=source.Version{continue}
		if !equalAssemblyDebianJSON(existing,source){return errAssemblyClosureDomain}
		return nil
	}
	out.sources=append(out.sources,source)
	return nil
}
