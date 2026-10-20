package api

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// These roles are selected by the independently configured producer profile.
// A document in one of these roles is input data, never an inspection receipt.
const (
	assemblyDependencyClosureRole = "assembly-dependency-closure"
	assemblyInstalledFilesRole = "assembly-installed-files"
	assemblyNoticesSourcesRole = "assembly-notices-sources"
	assemblyLuciHelperRole = "assembly-luci-helper"
	assemblyBaseRootfsRole = "assembly-base-rootfs"
	assemblyWheelsRole = "assembly-wheels"
	assemblyDebianSourcesRole = "assembly-debian-sources"
	assemblyRetainedSourceMaxBytes int64 = backend.DocumentDomainInputFileBytes
	assemblyRetainedSourcesMaxBytes int64 = backend.DocumentDomainInputTotalBytes
)

var errAssemblyClosureDomain = errors.New("document assembly input domain is incomplete or inconsistent")
var assemblyRelationAtom = regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]{0,199})(?::(any|native|amd64))?(?:[ \t]*\([ \t]*(<<|<=|=|>=|>>)[ \t]*([^() \t]+)[ \t]*\))?$`)

// Only captureDocumentAssemblyDomains, the concrete same-owner inspection
// constructor, supplies this handoff. None of these fields is decoded as an
// accepted receipt. Pure relationship checks below do not create the handoff.
type assemblyObservedDomains struct {
	plan *debianProducerPlan
	binary *debianBinarySemanticCapture
	base assemblyObservedBase
	wheels []assemblyObservedWheel
	sources []assemblyObservedSource
	installed []assemblyObservedFile
	notices []assemblyObservedNotice
	helper assemblyObservedHelper
	trust assemblyObservedTrust
	// Actual retained input/output guards. The producer retains all originals
	// and aliases; catalog publication does not transfer or close these tokens.
	guards []backend.DocumentImageAssemblyEvidenceGuard
	domains backend.DocumentDomainInputs
}

type assemblyObservedTrust struct {
	Policy assemblyDebianRootPolicyV1 `json:"policy"`
	Keys assemblyDebianKeyObservationsV1 `json:"keys"`
	Signatures assemblyDebianReleaseSignaturesV1 `json:"signatures"`
	Pins assemblyDebianProducerPins `json:"pins"`
}

type assemblyObservedBase struct {
	Layer backend.DocumentImageAssemblyInput `json:"layer"`
	Packages []assemblyDependencyPackage `json:"packages"`
}

type assemblyObservedWheel struct {
	ID string `json:"id"`
	Version string `json:"version"`
	InstalledOwner string `json:"installedOwner"`
	Archive backend.DocumentImageAssemblyInput `json:"archive"`
	Components []assemblyObservedNativeComponent `json:"components"`
	RequiresDist []string `json:"requiresDist"`
	RequiresPython string `json:"requiresPython"`
	Tags []string `json:"tags"`
	RootIsPurelib *bool `json:"rootIsPurelib"`
	EntryPoints []assemblyObservedWheelEntryPoint `json:"entryPoints"`
}

type assemblyObservedWheelEntryPoint struct {Group string `json:"group"`;Name string `json:"name"`;Value string `json:"value"`}

type assemblyObservedNativeComponent struct {
	ID string `json:"id"`
	Path string `json:"path"`
	Digest string `json:"digest"`
	Version string `json:"version"`
	Sources []string `json:"sources"`
}

// Source IDs are qualified (debian/name, wheel/name or luci). Components bind
// exact installed distribution owners; Built-Using adds a required source even
// when it is not the binary's primary Source. The actual signed paragraph and
// retained archive verification are established by the inspection constructor.
type assemblyObservedSource struct {
	ID string `json:"id"`
	Version string `json:"version"`
	Archives []backend.DocumentImageAssemblyInput `json:"archives"`
}

type assemblyObservedFile struct {
	Path string `json:"path"`
	Owner string `json:"owner"`
	Kind string `json:"kind"` // regular, directory or symlink; never a host path
	Bytes int64 `json:"bytes"`
	SHA256 string `json:"sha256"`
	Target string `json:"target"`
	Policy string `json:"policy"` // immutable, inspected Debian conffile, or exact generated metadata
}

type assemblyObservedNotice struct {
	Component string `json:"component"`
	File backend.DocumentImageAssemblyInput `json:"file"`
	Sources []string `json:"sources"` // qualified source ID + "@" + exact version
}

type assemblyObservedHelper struct {
	SourceCommit string `json:"sourceCommit"`
	SourceDigest string `json:"sourceDigest"`
	HelperDigest string `json:"helperDigest"`
	RecipeDigest string `json:"recipeDigest"`
	// This is the actual retained, profile-pinned source-retention document,
	// not a hash computed from a list of unobserved archive declarations.
	SourceRetentionDigest string `json:"sourceRetentionDigest"`
	Extractor backend.DocumentImageAssemblyInput `json:"extractor"`
	Verifier backend.DocumentImageAssemblyInput `json:"verifier"`
	Requirements backend.DocumentImageAssemblyInput `json:"requirements"`
	DependencyLock backend.DocumentImageAssemblyInput `json:"dependencyLock"`
}

func (d *assemblyObservedDomains) current(ctx context.Context,a *API) error {
	if d==nil||ctx==nil||ctx.Err()!=nil||d.plan==nil||d.binary==nil||d.binary.plan!=d.plan||d.binary.spool==nil||d.binary.spool.owner==nil||d.domains==nil {return errAssemblyClosureDomain}
	if err:=d.domains.CheckCurrent(ctx);err!=nil{return err}
	if err:=d.plan.current(ctx,a);err!=nil{return err}
	if !d.binary.spool.owner.positiveBinaryCompletion(d.binary.child)||!d.binary.spool.current(ctx){return errAssemblyClosureDomain}
	for _,guard:=range d.guards {if guard==nil{return errAssemblyClosureDomain};if err:=guard.CheckCurrent(ctx);err!=nil{return err}}
	if err:=d.plan.current(ctx,a);err!=nil{return err};return d.domains.CheckCurrent(ctx)
}

type assemblyDependencyPackage struct {
	Name string `json:"name"`
	Version string `json:"version"`
	Architecture string `json:"architecture"`
	MultiArch string `json:"multiArch"`
	Depends string `json:"depends"`
	PreDepends string `json:"preDepends"`
	Provides string `json:"provides"`
	Breaks string `json:"breaks"`
	Conflicts string `json:"conflicts"`
	Source string `json:"source"`
	BuiltUsing string `json:"builtUsing"`
	StaticBuiltUsing string `json:"staticBuiltUsing"`
}

type assemblyDependencyEdge struct {
	Package string `json:"package"`
	Field string `json:"field"`
	Group int `json:"group"`
	Alternative int `json:"alternative"`
	Provider string `json:"provider"`
	Version string `json:"version"`
}

type assemblyDependencyClosureV1 struct {
	Schema int `json:"schema"`
	Packages []assemblyDependencyPackage `json:"packages"`
	Edges []assemblyDependencyEdge `json:"edges"`
}

type assemblyRelation struct { name, architecture, operator, version string }

// Binary control relations have already had physical continuation whitespace
// unfolded by the retained binary decoder. Source-only build profiles and
// architecture restriction lists are not executable binary dependencies here.
func parseAssemblyRelations(value string) ([][]assemblyRelation, error) {
	if len(value)>65536 || strings.ContainsAny(value,"\x00\r\n") { return nil,errAssemblyClosureDomain }
	if value=="" { return nil,nil }
	groups:=strings.Split(value,",")
	if len(groups)>512 { return nil,errAssemblyClosureDomain }
	out:=make([][]assemblyRelation,len(groups))
	for i,group:=range groups {
		alternatives:=strings.Split(group,"|")
		if len(alternatives)>32 { return nil,errAssemblyClosureDomain }
		for _,alternative:=range alternatives {
			match:=assemblyRelationAtom.FindStringSubmatch(strings.TrimSpace(alternative))
			if match==nil || match[4]!=""&&!validAssemblyDebianVersion(match[4]) { return nil,errAssemblyClosureDomain }
			out[i]=append(out[i],assemblyRelation{match[1],match[2],match[3],match[4]})
		}
	}
	return out,nil
}

func validAssemblyDebianVersion(value string) bool {
	if len(value)==0 || len(value)>256 { return false }
	version:=value
	if colon:=strings.IndexByte(value,':');colon>=0 {
		if colon==0 { return false }
		for _,c:=range value[:colon] { if c<'0'||c>'9' { return false } }
		if _,err:=strconv.ParseUint(value[:colon],10,64);err!=nil { return false }
		version=value[colon+1:]
	}
	if len(version)==0 || version[0]<'0'||version[0]>'9' { return false }
	for _,c:=range version { if !(c>='0'&&c<='9'||c>='a'&&c<='z'||c>='A'&&c<='Z'||strings.ContainsRune(".+~-:",c)) { return false } }
	// A Debian revision, when present, must be nonempty and cannot contain '-'.
	if dash:=strings.LastIndexByte(version,'-');dash>=0 {
		if dash==len(version)-1 { return false }
		for _,c:=range version[dash+1:] { if !(c>='0'&&c<='9'||c>='a'&&c<='z'||c>='A'&&c<='Z'||strings.ContainsRune(".+~",c)) { return false } }
	}
	return true
}

func splitAssemblyDebianVersion(value string)(uint64,string,string) {
	epoch:=uint64(0)
	if colon:=strings.IndexByte(value,':');colon>=0 { epoch,_=strconv.ParseUint(value[:colon],10,64);value=value[colon+1:] }
	upstream,revision:=value,"0"
	if dash:=strings.LastIndexByte(value,'-');dash>=0 { upstream,revision=value[:dash],value[dash+1:] }
	return epoch,upstream,revision
}

func assemblyVersionOrder(c byte)int {
	if c=='~' { return -1 };if c==0 { return 0 }
	if c>='A'&&c<='Z'||c>='a'&&c<='z' { return int(c) }
	return int(c)+256
}

// Debian's ordering compares non-digit segments (~ before even end-of-string),
// then arbitrarily long digit runs numerically without machine-integer casts.
func compareAssemblyVersionPart(a,b string)int {
	i,j:=0,0
	digit:=func(c byte)bool{return c>='0'&&c<='9'}
	for i<len(a)||j<len(b) {
		for i<len(a)&&!digit(a[i])||j<len(b)&&!digit(b[j]) {
			var ac,bc byte
			if i<len(a) { ac=a[i] };if j<len(b) { bc=b[j] }
			if digit(ac) { ac=0 };if digit(bc) { bc=0 }
			if x,y:=assemblyVersionOrder(ac),assemblyVersionOrder(bc);x!=y { if x<y{return -1};return 1 }
			if i<len(a)&&!digit(a[i]) {i++};if j<len(b)&&!digit(b[j]) {j++}
		}
		for i<len(a)&&a[i]=='0' {i++};for j<len(b)&&b[j]=='0' {j++}
		x,y:=i,j
		for i<len(a)&&digit(a[i]) {i++};for j<len(b)&&digit(b[j]) {j++}
		if i-x!=j-y {if i-x<j-y{return -1};return 1}
		if part:=strings.Compare(a[x:i],b[y:j]);part!=0 {return part}
	}
	return 0
}

func compareAssemblyDebianVersion(a,b string)int {
	ae,au,ar:=splitAssemblyDebianVersion(a);be,bu,br:=splitAssemblyDebianVersion(b)
	if ae!=be {if ae<be{return -1};return 1}
	if c:=compareAssemblyVersionPart(au,bu);c!=0 {return c}
	return compareAssemblyVersionPart(ar,br)
}

func assemblyRelationVersionMatches(relation assemblyRelation,version string)bool {
	if relation.operator=="" { return true };if version=="" {return false}
	c:=compareAssemblyDebianVersion(version,relation.version)
	switch relation.operator {case "<<":return c<0;case "<=":return c<=0;case "=":return c==0;case ">=":return c>=0;case ">>":return c>0};return false
}

func assemblyPackageProvides(p assemblyDependencyPackage,r assemblyRelation)(string,bool) {
	if r.architecture=="any"&&p.MultiArch!="allowed" {return "",false}
	if p.Name==r.name&&assemblyRelationVersionMatches(r,p.Version) {return p.Version,true}
	provides,err:=parseAssemblyRelations(p.Provides);if err!=nil {return "",false}
	for _,group:=range provides { for _,v:=range group {
		if v.name==r.name&&v.architecture==""&&(v.operator==""||v.operator=="=")&&assemblyRelationVersionMatches(r,v.version) {return v.version,true}
	} }
	return "",false
}

// The universe must be made from actual retained binary control observations
// plus an inspected base-rootfs status inventory. A lock's expected versions
// are not a substitute for the latter. This pure constructor proves only the
// dependency domain; it cannot mint source/archive/native completion proof.
func deriveAssemblyDependencyClosure(packages []assemblyDependencyPackage)(assemblyDependencyClosureV1,error) {
	var empty assemblyDependencyClosureV1
	if len(packages)==0||len(packages)>512 {return empty,errAssemblyClosureDomain}
	rows:=append([]assemblyDependencyPackage(nil),packages...)
	sort.Slice(rows,func(i,j int)bool{return rows[i].Name<rows[j].Name})
	total:=0
	for i,p:=range rows {
		if !assemblyRelationAtom.MatchString(p.Name)||strings.ContainsAny(p.Name,":() \t")||!validAssemblyDebianVersion(p.Version)||p.Architecture!="amd64"&&p.Architecture!="all"||i>0&&rows[i-1].Name==p.Name {return empty,errAssemblyClosureDomain}
		if p.MultiArch!=""&&p.MultiArch!="no"&&p.MultiArch!="same"&&p.MultiArch!="foreign"&&p.MultiArch!="allowed" {return empty,errAssemblyClosureDomain}
		for _,value:=range []string{p.Depends,p.PreDepends,p.Provides,p.Breaks,p.Conflicts} {total+=len(value);if total>4<<20{return empty,errAssemblyClosureDomain};if _,err:=parseAssemblyRelations(value);err!=nil{return empty,err}}
		provides,_:=parseAssemblyRelations(p.Provides)
		for _,group:=range provides {if len(group)!=1||group[0].architecture!=""||group[0].operator!=""&&group[0].operator!="=" {return empty,errAssemblyClosureDomain}}
	}
	out:=assemblyDependencyClosureV1{Schema:1,Packages:rows,Edges:[]assemblyDependencyEdge{}}
	for _,p:=range rows {
		for _,field:=range []struct{name,value string}{{"pre-depends",p.PreDepends},{"depends",p.Depends}} {
			groups,_:=parseAssemblyRelations(field.value)
			for index,alternatives:=range groups {
				found:=false
				for alt,r:=range alternatives {
					for _,provider:=range rows {if version,ok:=assemblyPackageProvides(provider,r);ok {out.Edges=append(out.Edges,assemblyDependencyEdge{p.Name,field.name,index,alt,provider.Name,version});found=true;break}}
					if found {break}
				}
				if !found||len(out.Edges)>8192{return empty,errAssemblyClosureDomain}
			}
		}
		for _,value:=range []string{p.Breaks,p.Conflicts} {
			groups,_:=parseAssemblyRelations(value)
			for _,group:=range groups {if len(group)!=1{return empty,errAssemblyClosureDomain};for _,other:=range rows {if other.Name!=p.Name {if _,hit:=assemblyPackageProvides(other,group[0]);hit{return empty,errAssemblyClosureDomain}}}}
		}
	}
	return out,nil
}

func assemblyDependencyFromControl(fields assemblyDebianBinaryFieldsV1)(assemblyDependencyPackage,error) {
	if fields.Package==nil||fields.Version==nil||fields.Architecture==nil{return assemblyDependencyPackage{},errAssemblyClosureDomain}
	text:=func(v *string)string{if v==nil{return ""};return *v}
	p:=assemblyDependencyPackage{Name:*fields.Package,Version:*fields.Version,Architecture:*fields.Architecture,MultiArch:text(fields.MultiArch),Depends:text(fields.Depends),PreDepends:text(fields.PreDepends),Provides:text(fields.Provides),Breaks:text(fields.Breaks),Conflicts:text(fields.Conflicts),Source:text(fields.Source),BuiltUsing:text(fields.BuiltUsing),StaticBuiltUsing:text(fields.StaticBuiltUsing)}
	// Validate syntax without pretending a one-package universe is closed.
	if !validAssemblyDebianVersion(p.Version)||p.Architecture!="amd64"&&p.Architecture!="all" {return assemblyDependencyPackage{},errAssemblyClosureDomain}
	return p,nil
}

func assemblyClosureSafePath(value string) bool {
	if value==""||len(value)>1024||!utf8.ValidString(value)||strings.HasPrefix(value,"/")||path.Clean(value)!=value||value=="."||value==".."||strings.HasPrefix(value,"../")||strings.ContainsAny(value,"\\:\x00\r\n") {return false}
	for _,c:=range value {if c<32||c==127{return false}}
	return true
}

func assemblyClosureInputValid(input backend.DocumentImageAssemblyInput)bool {
	return assemblyClosureSafePath(input.Path)&&input.Bytes>0&&input.Bytes<=backend.DocumentImageAssemblyInputMaxBytes&&assemblyCatalogHex(input.Digest)
}

func assemblyClosureComponent(value string)bool {
	if value=="luci"||value=="base/rootfs"{return true}
	parts:=strings.Split(value,"/")
	if len(parts)<2||len(parts)>4||parts[0]!="debian"&&parts[0]!="wheel" {return false}
	for _,part:=range parts[1:] {if part==""||len(part)>200||strings.Trim(part,"abcdefghijklmnopqrstuvwxyz0123456789+._-")!=""||part=="."||part==".."{return false}}
	return true
}

func assemblyPackageSourceRequirements(p assemblyDependencyPackage)([]string,error) {
	primaryName,primaryVersion:=p.Name,p.Version
	if p.Source!="" {
		match:=regexp.MustCompile(`^([a-z0-9][a-z0-9+.-]{0,199})(?:[ \t]+\(([^() \t]+)\))?$`).FindStringSubmatch(p.Source)
		if match==nil{return nil,errAssemblyClosureDomain};primaryName=match[1];if match[2]!="" {primaryVersion=match[2]}
	}
	if !validAssemblyDebianVersion(primaryVersion){return nil,errAssemblyClosureDomain}
	out:=[]string{"debian/"+primaryName+"@"+primaryVersion}
	for _,field:=range []string{p.BuiltUsing,p.StaticBuiltUsing} {
		groups,err:=parseAssemblyRelations(field);if err!=nil{return nil,err}
		for _,group:=range groups {if len(group)!=1||group[0].architecture!=""||group[0].operator!="=" {return nil,errAssemblyClosureDomain};out=append(out,"debian/"+group[0].name+"@"+group[0].version)}
	}
	sort.Strings(out)
	unique:=out[:0];for _,v:=range out {if len(unique)==0||unique[len(unique)-1]!=v {unique=append(unique,v)}}
	return unique,nil
}

func assemblyNoticeSourcesCover(required,provided []string,available map[string]bool)bool {
	if len(required)==0||len(provided)==0||len(provided)>128{return false}
	seen:=map[string]bool{};for _,source:=range provided {if !available[source]||seen[source]{return false};seen[source]=true}
	for _,source:=range required {if !seen[source]{return false}}
	return true
}

// All input slices here come from the concrete inspection handoff, not decoded
// profile claims. This joins independently observed domains and checks their
// exact relationships before any assembly acceptance flags can be derived.
func validateAssemblyObservedDomains(d *assemblyObservedDomains,source assemblyCatalogSource,packages []assemblyDependencyPackage)(assemblyDependencyClosureV1,error) {
	var empty assemblyDependencyClosureV1
	if d==nil||!validAssemblyCatalogSource(source)||len(packages)!=174||len(d.base.Packages)!=88||len(d.wheels)!=2||len(d.sources)==0||len(d.sources)>1024||len(d.installed)==0||len(d.installed)>100000||len(d.notices)==0||len(d.notices)>512{return empty,errAssemblyClosureDomain}
	if h:=d.helper;h.SourceCommit!=source.SourceCommit||h.SourceDigest!=source.SourceDigest||h.HelperDigest!=source.HelperDigest||h.RecipeDigest!=source.RecipeDigest||h.SourceRetentionDigest!=source.SourceRetentionDigest||h.DependencyLock.Digest!=source.DependencyLockDigest{return empty,errAssemblyClosureDomain}
	merged:=map[string]assemblyDependencyPackage{}
	for _,p:=range d.base.Packages {if _,exists:=merged[p.Name];exists{return empty,errAssemblyClosureDomain};merged[p.Name]=p}
	selected:=map[string]bool{}
	for _,p:=range packages {if selected[p.Name]{return empty,errAssemblyClosureDomain};selected[p.Name]=true;merged[p.Name]=p}
	universe:=make([]assemblyDependencyPackage,0,len(merged));for _,p:=range merged {universe=append(universe,p)}
	dependency,err:=deriveAssemblyDependencyClosure(universe);if err!=nil{return empty,err}
	if err=validateAssemblyPythonWheels(d.wheels,dependency.Packages);err!=nil{return empty,err}
	required:=map[string][]string{}
	for _,p:=range dependency.Packages {requirements,e:=assemblyPackageSourceRequirements(p);if e!=nil{return empty,e};required["debian/"+p.Name]=requirements}
	natives:=map[string]assemblyObservedNativeComponent{}
	for _,wheel:=range d.wheels {
		if !assemblyClosureComponent(wheel.ID)||!strings.HasPrefix(wheel.ID,"wheel/")||len(strings.Split(wheel.ID,"/"))!=2||wheel.Version==""||len(wheel.Version)>128||strings.ContainsAny(wheel.Version,"@\x00\r\n")||!assemblyClosureInputValid(wheel.Archive)||len(wheel.Components)==0||len(wheel.Components)>128{return empty,errAssemblyClosureDomain}
		if _,exists:=required[wheel.ID];exists{return empty,errAssemblyClosureDomain}
		required[wheel.ID]=[]string{wheel.ID+"@"+wheel.Version}
		seen:=map[string]bool{}
		for _,component:=range wheel.Components {
			if !assemblyClosureSafePath(component.Path)||component.ID!=wheel.ID+"/native-"+assemblyCatalogDigest([]byte(component.Path))[:16]||!assemblyCatalogHex(component.Digest)||component.Version==""||len(component.Version)>256||strings.ContainsAny(component.Version,"@\x00\r\n")||len(component.Sources)==0||len(component.Sources)>32||seen[component.ID]{return empty,errAssemblyClosureDomain}
			seen[component.ID]=true;natives[component.ID]=component;required[component.ID]=append([]string(nil),component.Sources...)
		}
	}
	required["luci"]=[]string{"luci@"+source.SourceCommit}
	for _,file:=range d.installed {if file.Owner=="base/rootfs"{required["base/rootfs"]=[]string{"base/rootfs@"+d.base.Layer.Digest};break}}
	sources:=map[string]bool{};sourcePaths:=map[string]backend.DocumentImageAssemblyInput{};sourceBytes:=int64(0)
	for _,s:=range d.sources {
		if !assemblyClosureComponent(s.ID)||s.Version==""||len(s.Version)>256||strings.ContainsAny(s.Version,"@\x00\r\n")||len(s.Archives)==0||len(s.Archives)>16{return empty,errAssemblyClosureDomain}
		key:=s.ID+"@"+s.Version;if sources[key]{return empty,errAssemblyClosureDomain};sources[key]=true
		for _,archive:=range s.Archives {
			// Corresponding source stays retained under its separate guarded
			// streaming budget. It is never an assembly payload/memory allowance.
			if !assemblyClosureSafePath(archive.Path)||archive.Bytes<=0||archive.Bytes>assemblyRetainedSourceMaxBytes||!assemblyCatalogHex(archive.Digest){return empty,errAssemblyClosureDomain}
			if old,exists:=sourcePaths[archive.Path];exists {if old!=archive{return empty,errAssemblyClosureDomain}}else{if archive.Bytes>assemblyRetainedSourcesMaxBytes-sourceBytes{return empty,errAssemblyClosureDomain};sourceBytes+=archive.Bytes}
			sourcePaths[archive.Path]=archive
		}
	}
	noticed:=map[string]bool{};noticePaths:=map[string]backend.DocumentImageAssemblyInput{}
	usedSources:=map[string]bool{}
	for _,notice:=range d.notices {
		needs,exists:=required[notice.Component]
		if !exists||!assemblyClosureInputValid(notice.File)||notice.File.Bytes>1<<20||!strings.HasPrefix(notice.File.Path,"notices/")||!assemblyNoticeSourcesCover(needs,notice.Sources,sources){return empty,errAssemblyClosureDomain}
		if old,exists:=noticePaths[notice.File.Path];exists&&old!=notice.File{return empty,errAssemblyClosureDomain};noticePaths[notice.File.Path]=notice.File
		for _,id:=range notice.Sources {usedSources[id]=true}
		noticed[notice.Component]=true
	}
	for component:=range required {if !noticed[component]{return empty,errAssemblyClosureDomain}}
	for sourceID:=range sources {if !usedSources[sourceID]{return empty,errAssemblyClosureDomain}}
	paths:=map[string]assemblyObservedFile{};metadataBytes:=0;totalBytes:=int64(0)
	for _,file:=range d.installed {
		if !assemblyClosureSafePath(file.Path)||!assemblyClosureComponent(file.Owner)||!noticed[file.Owner]{return empty,errAssemblyClosureDomain}
		if _,exists:=paths[file.Path];exists{return empty,errAssemblyClosureDomain};paths[file.Path]=file
		metadataBytes+=len(file.Path)+len(file.Owner)+len(file.Target)+len(file.SHA256)+64;if metadataBytes>32<<20{return empty,errAssemblyClosureDomain}
		if file.Policy!=""&&file.Policy!="immutable" {
			if file.Policy=="generated-alternative" {if file.Kind!="symlink"||!strings.HasPrefix(file.Path,"etc/alternatives/")||strings.Contains(strings.TrimPrefix(file.Path,"etc/alternatives/"),"/"){return empty,errAssemblyClosureDomain}}else if file.Policy=="wheel-installer-metadata" {if file.Kind!="regular"||!assemblyWheelMetadataPath(file.Path,file.Owner,d.wheels){return empty,errAssemblyClosureDomain}}else if file.Kind!="regular"||file.Policy=="debian-conffile"&&!strings.HasPrefix(file.Owner,"debian/")||file.Policy=="generated-metadata"&&!assemblyGeneratedMetadataPath(file.Path,merged)||file.Policy!="debian-conffile"&&file.Policy!="generated-metadata" {return empty,errAssemblyClosureDomain}
		}
		switch file.Kind {
		case "regular":if file.Bytes<0||file.Bytes>backend.DocumentImageAssemblyOutputMaxBytes-totalBytes||!assemblyCatalogHex(file.SHA256)||file.Target!=""||file.Bytes==0&&file.SHA256!=assemblyCatalogDigest(nil){return empty,errAssemblyClosureDomain};totalBytes+=file.Bytes
		case "directory":if file.Bytes!=0||file.SHA256!=""||file.Target!=""{return empty,errAssemblyClosureDomain}
		case "symlink":if file.Bytes!=0||file.SHA256!=""||file.Target==""||len(file.Target)>1024||!utf8.ValidString(file.Target)||strings.ContainsAny(file.Target,"\\:\x00\r\n"){return empty,errAssemblyClosureDomain};target:=path.Clean(path.Join(path.Dir(file.Path),file.Target));if strings.HasPrefix(file.Target,"/"){target=strings.TrimPrefix(path.Clean(file.Target),"/")};if !assemblyClosureSafePath(target){return empty,errAssemblyClosureDomain}
		default:return empty,errAssemblyClosureDomain
		}
	}
	for id,native:=range natives {file,ok:=paths["usr/local/lib/python3.11/dist-packages/"+native.Path];if !ok||file.Owner!=id||file.Kind!="regular"||file.SHA256!=native.Digest||file.Policy!=""&&file.Policy!="immutable" {return empty,errAssemblyClosureDomain}}
	// This is an expected namespace, not a claim that every link resolves.
	// Debian slim can retain a dangling documentation link. Actual notice reads
	// must resolve a confined immutable target in the inspection constructor;
	// mere presence of a symlink here never supplies notice/source bytes.
	// Canonical domain records must remain bounded independently of native read
	// budgets. The eventual page writer enforces the smaller catalog per-page cap.
	for _,value:=range []any{d.base,d.wheels,d.sources,d.notices,d.helper,dependency} {body,e:=json.Marshal(value);if e!=nil||len(body)>16<<20{return empty,errAssemblyClosureDomain}}
	return dependency,nil
}

func assemblyGeneratedMetadataPath(name string,packages map[string]assemblyDependencyPackage)bool {
	switch name {
	case "var/lib/dpkg/status","var/lib/dpkg/status-old","var/lib/dpkg/available","var/lib/dpkg/available-old","var/lib/dpkg/lock","var/lib/dpkg/lock-frontend","var/lib/dpkg/triggers/File","var/lib/dpkg/triggers/Unincorp","var/lib/dpkg/triggers/Lock","etc/ld.so.cache","etc/passwd","etc/passwd-","etc/group","etc/group-","etc/shadow","etc/shadow-","etc/gshadow","etc/gshadow-","var/log/dpkg.log","var/log/alternatives.log":return true
	}
	if !strings.HasPrefix(name,"var/lib/dpkg/info/"){return false}
	stem:=strings.TrimPrefix(name,"var/lib/dpkg/info/");dot:=strings.LastIndexByte(stem,'.');if dot<0{return false}
	suffix:=stem[dot+1:];stem=strings.TrimSuffix(stem[:dot],":amd64")
	if _,ok:=packages[stem];!ok{return false}
	switch suffix {case "list","md5sums","conffiles","triggers","preinst","postinst","prerm","postrm","shlibs","symbols":return true};return false
}

func assemblyWheelMetadataPath(name,owner string,wheels []assemblyObservedWheel)bool {
	for _,wheel:=range wheels {if owner!=wheel.ID{continue};prefix:="usr/local/lib/python3.11/dist-packages/"+wheel.InstalledOwner+"/";if !strings.HasPrefix(name,prefix){continue};switch strings.TrimPrefix(name,prefix){case "RECORD","INSTALLER","REQUESTED","direct_url.json":return true}}
	return false
}

var assemblyPythonConstraint=regexp.MustCompile(`^(>=|<=|==|!=|>|<)([0-9]{1,5})(?:\.([0-9]{1,5}))?(?:\.([0-9]{1,5}|\*))?$`)
var assemblyInactiveExtraRequirement=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}(?:\[[A-Za-z0-9_,.-]+\])?(?:[ \t]*[!<>=~]{1,2}[ \t]*[0-9][A-Za-z0-9.*+!_,=<>~ \t-]*|[ \t]*\([!<>=~]{1,2}[ \t]*[0-9][A-Za-z0-9.*+!_,=<>~ \t-]*\))?[ \t]*;[ \t]*extra[ \t]*==[ \t]*(?:"[A-Za-z0-9._-]+"|'[A-Za-z0-9._-]+')[ \t]*$`)
var assemblyWheelEntryName=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var assemblyWheelEntryValue=regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*:[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// This is deliberately NOT a generic PEP440/508 implementation. The selected
// profile has two wheels and selects no extras. Unsupported metadata refuses
// rather than dropping an active dependency while pip runs with --no-deps.
func validateAssemblyPythonWheels(wheels []assemblyObservedWheel,packages []assemblyDependencyPackage)error {
	python:=[3]int{};found:=false;glibcMinor:=-1
	for _,p:=range packages {if p.Name=="python3.11" {_,upstream,_:=splitAssemblyDebianVersion(p.Version);pieces:=strings.Split(upstream,".");if len(pieces)!=3{return errAssemblyClosureDomain};for i,piece:=range pieces {n,e:=strconv.Atoi(piece);if e!=nil||n<0||n>65535{return errAssemblyClosureDomain};python[i]=n};found=true}}
	for _,p:=range packages {if p.Name=="libc6" {_,upstream,_:=splitAssemblyDebianVersion(p.Version);pieces:=strings.Split(upstream,".");if len(pieces)<2||pieces[0]!="2"{return errAssemblyClosureDomain};n,e:=strconv.Atoi(pieces[1]);if e!=nil{return errAssemblyClosureDomain};glibcMinor=n}}
	if !found||python[0]!=3||python[1]!=11||glibcMinor!=36||len(wheels)!=2{return errAssemblyClosureDomain}
	for _,wheel:=range wheels {
		if wheel.RootIsPurelib==nil||len(wheel.Tags)==0||len(wheel.Tags)>16||len(wheel.RequiresPython)>512||len(wheel.RequiresDist)>128||len(wheel.InstalledOwner)>255||!strings.HasSuffix(wheel.InstalledOwner,".dist-info")||strings.Contains(wheel.InstalledOwner,"/")||!assemblyClosureSafePath(wheel.InstalledOwner){return errAssemblyClosureDomain}
		seen:=map[string]bool{}
		compatible:=false
		for _,tag:=range wheel.Tags {if seen[tag]{return errAssemblyClosureDomain};seen[tag]=true;ok,e:=assemblyWheelTagCompatible(tag,python[1],glibcMinor);if e!=nil{return e};compatible=compatible||ok};if !compatible{return errAssemblyClosureDomain}
		if wheel.RequiresPython!="" {for _,clause:=range strings.Split(wheel.RequiresPython,",") {
			match:=assemblyPythonConstraint.FindStringSubmatch(strings.TrimSpace(clause));if match==nil{return errAssemblyClosureDomain}
			want:=[3]int{};for i:=0;i<3;i++ {if match[i+2]!=""&&match[i+2]!="*"{want[i],_=strconv.Atoi(match[i+2])}}
			limit:=3;if match[4]=="*" {if match[1]!="=="&&match[1]!="!="{return errAssemblyClosureDomain};limit=2}
			comparison:=0;for i:=0;i<limit;i++ {if python[i]<want[i]{comparison=-1;break};if python[i]>want[i]{comparison=1;break}}
			ok:=false;switch match[1] {case ">=":ok=comparison>=0;case "<=":ok=comparison<=0;case ">":ok=comparison>0;case "<":ok=comparison<0;case "==":ok=comparison==0;case "!=":ok=comparison!=0};if !ok{return errAssemblyClosureDomain}
		} }
		for _,requirement:=range wheel.RequiresDist {if len(requirement)>4096||!assemblyInactiveExtraRequirement.MatchString(requirement){return errAssemblyClosureDomain}}
		if len(wheel.EntryPoints)>128{return errAssemblyClosureDomain};entries:=map[string]bool{}
		for _,entry:=range wheel.EntryPoints {if entry.Group!="console_scripts"&&entry.Group!="gui_scripts"||!assemblyWheelEntryName.MatchString(entry.Name)||!assemblyWheelEntryValue.MatchString(entry.Value)||entries[entry.Name]{return errAssemblyClosureDomain};entries[entry.Name]=true}
	}
	return nil
}

func assemblyWheelTagCompatible(tag string,pythonMinor,glibcMinor int)(bool,error) {
	parts:=strings.Split(tag,"-");if len(tag)>512||len(parts)!=3{return false,errAssemblyClosureDomain}
	interpreters,abis,platforms:=strings.Split(parts[0],"."),strings.Split(parts[1],"."),strings.Split(parts[2],".")
	if len(interpreters)>4||len(abis)>4||len(platforms)>4{return false,errAssemblyClosureDomain}
	compatible:=false
	for _,platform:=range platforms {
		minor:=-1
		if platform=="manylinux2014_x86_64"{minor=17}else{match:=regexp.MustCompile(`^manylinux_2_([0-9]{1,2})_x86_64$`).FindStringSubmatch(platform);if match==nil{return false,errAssemblyClosureDomain};minor,_=strconv.Atoi(match[1]);if minor<17||minor>36{return false,errAssemblyClosureDomain}}
		for _,interpreter:=range interpreters {for _,abi:=range abis {
			if interpreter=="py3" {if abi!="none"{return false,errAssemblyClosureDomain};compatible=compatible||minor<=glibcMinor;continue}
			match:=regexp.MustCompile(`^cp3([0-9]{1,2})$`).FindStringSubmatch(interpreter);if match==nil{return false,errAssemblyClosureDomain};required,_:=strconv.Atoi(match[1]);if required<6||required>13{return false,errAssemblyClosureDomain}
			if abi=="abi3" {compatible=compatible||required<=pythonMinor&&minor<=glibcMinor}else if abi==interpreter {compatible=compatible||required==pythonMinor&&minor<=glibcMinor}else{return false,errAssemblyClosureDomain}
		} }
	}
	return compatible,nil
}
