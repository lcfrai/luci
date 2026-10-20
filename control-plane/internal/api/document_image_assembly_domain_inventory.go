package api

import (
	"path"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/documentinputs"
)

type assemblyInputInventory struct {
	files map[string]assemblyObservedFile
	notices map[string]map[string]int64
	packageNames map[string]bool
	hasBaseOwner bool
}

func newAssemblyInputInventory()*assemblyInputInventory {
	return &assemblyInputInventory{files:map[string]assemblyObservedFile{},notices:map[string]map[string]int64{},packageNames:map[string]bool{}}
}

func (s *assemblyInputInventory) notice(component,digest string,bytes int64) {
	if s.notices[component]==nil{s.notices[component]=map[string]int64{}}
	s.notices[component][digest]=bytes
}

func (s *assemblyInputInventory) noticeMatches(component,digest string,bytes int64)bool {
	amount,ok:=s.notices[component][digest];return ok&&amount==bytes
}

func (s *assemblyInputInventory) addBase(base documentinputs.Base)error {
	for _,pkg:=range base.Packages{s.packageNames[pkg.Name]=true}
	for _,file:=range base.Files {
		owner:=strings.TrimSuffix(file.Owner,":amd64")
		if file.Kind=="directory"&&owner==""{continue}
		component:="base/rootfs"
		if s.packageNames[owner]{component="debian/"+owner}
		if err:=s.put(file,component,"",false);err!=nil{return err}
	}
	for _,notice:=range base.Notices {
		s.notice("base/rootfs",notice.SHA256,notice.Bytes)
	}
	return s.materializeHardlinks()
}

func (s *assemblyInputInventory) addDeb(deb documentinputs.Deb)error {
	s.packageNames[deb.Package.Name]=true
	conffiles:=map[string]bool{}
	for _,name:=range deb.Conffiles{conffiles[strings.TrimPrefix(name,"/")]=true}
	for _,file:=range deb.Files {
		policy:="";if conffiles[file.Path]{policy="debian-conffile"}
		if err:=s.put(file,"debian/"+deb.Package.Name,policy,true);err!=nil{return err}
	}
	return s.materializeHardlinks()
}

func (s *assemblyInputInventory) addWheel(selected assemblyWheelSelection,wheel documentinputs.Wheel)([]assemblyObservedNativeComponent,error) {
	actual:=map[string]documentinputs.Component{}
	for _,component:=range wheel.Components{if _,ok:=actual[component.Path];ok{return nil,errAssemblyClosureDomain};actual[component.Path]=component}
	if len(actual)==0||len(actual)!=len(selected.Native)||len(actual)>128{return nil,errAssemblyClosureDomain}
	files:=map[string]documentinputs.File{}
	for _,file:=range wheel.Files{files[file.Path]=file}
	owners:=map[string]string{}
	components:=append([]assemblyObservedNativeComponent(nil),selected.Native...)
	for _,binding:=range components {
		component,ok:=actual[binding.Path]
		file,exists:=files[binding.Path]
		id:=selected.ID+"/native-"+assemblyCatalogDigest([]byte(binding.Path))[:16]
		if !ok||!exists||component.Architecture!="amd64"||file.Kind!="regular"||file.SHA256!=binding.Digest||
			binding.ID!=id||binding.Version==""||len(binding.Sources)==0||owners[binding.Path]!=""{return nil,errAssemblyClosureDomain}
		owners[binding.Path]=id
	}
	for _,file:=range wheel.Files {
		owner:=selected.ID;if native:=owners[file.Path];native!=""{owner=native}
		policy:=""
		if path.Dir(file.Path)==selected.InstalledOwner {
			switch path.Base(file.Path) {
			case "RECORD","INSTALLER","REQUESTED","direct_url.json":policy="wheel-installer-metadata"
			}
		}
		name,err:=assemblyWheelInstalledPath(file.Path);if err!=nil{return nil,err}
		file.Path=name
		if err=s.put(file,owner,policy,false);err!=nil{return nil,err}
	}
	// Exact original notices are retained for the wheel and each explicitly
	// bound native component. Required source relationships are checked by the
	// closure algebra; sharing a grant does not waive a component's sources.
	for _,notice:=range wheel.Notices {
		s.notice(selected.ID,notice.SHA256,notice.Bytes)
		for _,component:=range components{s.notice(component.ID,notice.SHA256,notice.Bytes)}
	}
	return components,nil
}

func (s *assemblyInputInventory) put(file documentinputs.File,owner,policy string,replace bool)error {
	if len(s.files)>=100000||!assemblyClosureSafePath(file.Path){return errAssemblyClosureDomain}
	name,err:=s.resolve(file.Path,false);if err!=nil{return err}
	value:=assemblyObservedFile{Path:name,Owner:owner,Kind:file.Kind,Bytes:file.Bytes,SHA256:file.SHA256,Target:file.Target,Policy:policy}
	if old,ok:=s.files[name];ok&&!replace {
		if old.Kind=="directory"&&value.Kind=="directory"{return nil}
		return errAssemblyClosureDomain
	}
	s.files[name]=value
	return nil
}

// Resolve only archive namespace links. No host path is opened here. A link
// target escaping the image or a cycle is refused before it can become a
// realized inventory file or a notice binding.
func (s *assemblyInputInventory) resolve(name string,final bool)(string,error) {
	if !assemblyClosureSafePath(name){return "",errAssemblyClosureDomain}
	for hops:=0;hops<64;hops++ {
		parts:=strings.Split(name,"/");changed:=false
		limit:=len(parts);if !final{limit--}
		for i:=1;i<=limit;i++ {
			prefix:=strings.Join(parts[:i],"/");file,ok:=s.files[prefix]
			if !ok{continue}
			if file.Kind!="symlink" {
				if i<len(parts)&&file.Kind!="directory"{return "",errAssemblyClosureDomain}
				continue
			}
			target:=file.Target
			if strings.HasPrefix(target,"/"){target=strings.TrimPrefix(path.Clean(target),"/")}else{target=path.Join(path.Dir(prefix),target)}
			if i<len(parts){target=path.Join(target,strings.Join(parts[i:],"/"))}
			if !assemblyClosureSafePath(target){return "",errAssemblyClosureDomain}
			name=target;changed=true;break
		}
		if !changed{return name,nil}
	}
	return "",errAssemblyClosureDomain
}

func (s *assemblyInputInventory) materializeHardlinks()error {
	// Resolve hardlink content from the actual same archive namespace. An
	// unresolved link is never assigned an invented digest.
	for pass:=0;pass<64;pass++ {
		pending,changed:=false,false
		for name,file:=range s.files {
			if file.Kind!="hardlink"{continue};pending=true
			target:=strings.TrimPrefix(file.Target,"./")
			if !assemblyClosureSafePath(target){return errAssemblyClosureDomain}
			resolved,err:=s.resolve(target,true);if err!=nil{return err}
			actual,ok:=s.files[resolved];if !ok{return errAssemblyClosureDomain}
			if actual.Kind=="hardlink"{continue}
			if actual.Kind!="regular"{return errAssemblyClosureDomain}
			file.Kind,file.Bytes,file.SHA256,file.Target="regular",actual.Bytes,actual.SHA256,""
			s.files[name]=file;changed=true
		}
		if !pending{break};if !changed||pass==63{return errAssemblyClosureDomain}
	}
	return nil
}

func (s *assemblyInputInventory) finish(packages map[string]assemblyDependencyPackage)error {
	// Debian package copyright can itself be a confined link. Resolve it to
	// actual inventoried bytes before accepting a copied original grant.
	for name:=range packages {
		delete(s.notices,"debian/"+name)
		target,err:=s.resolve("usr/share/doc/"+name+"/copyright",true);if err!=nil{return err}
		if actual,ok:=s.files[target];ok&&actual.Kind=="regular"{s.notice("debian/"+name,actual.SHA256,actual.Bytes)}
	}
	for name,file:=range s.files {
		if file.Kind=="regular"&&assemblyGeneratedMetadataPath(name,packages){file.Policy="generated-metadata"}
		if file.Kind=="symlink"&&strings.HasPrefix(name,"etc/alternatives/")&&!strings.Contains(strings.TrimPrefix(name,"etc/alternatives/"),"/"){file.Policy="generated-alternative"}
		if file.Owner=="base/rootfs"{s.hasBaseOwner=true}
		s.files[name]=file
	}
	return nil
}

func(s *assemblyInputInventory)sorted()[]assemblyObservedFile {
	out:=make([]assemblyObservedFile,0,len(s.files));for _,file:=range s.files{out=append(out,file)}
	sort.Slice(out,func(i,j int)bool{return out[i].Path<out[j].Path})
	return out
}
