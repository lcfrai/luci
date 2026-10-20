package api

import (
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/documentinputs"
)

func TestAssemblyInventoryRetainsMaterializedHardlinkAcrossUpgrade(t *testing.T) {
	s:=newAssemblyInputInventory()
	old:=assemblyCatalogDigest([]byte("old"))
	next:=assemblyCatalogDigest([]byte("new"))
	base:=documentinputs.Base{Packages:[]documentinputs.Package{{Name:"sample",Version:"1",Architecture:"amd64"}},
		Files:[]documentinputs.File{
			{Path:"usr/lib/target",Owner:"sample",Kind:"regular",Bytes:3,SHA256:old},
			{Path:"usr/lib/alias",Owner:"sample",Kind:"hardlink",Target:"usr/lib/target"},
		}}
	if err:=s.addBase(base);err!=nil{t.Fatal(err)}
	if err:=s.addDeb(documentinputs.Deb{Package:documentinputs.Package{Name:"sample",Version:"2",Architecture:"amd64"},
		Files:[]documentinputs.File{{Path:"usr/lib/target",Kind:"regular",Bytes:3,SHA256:next}}});err!=nil{t.Fatal(err)}
	if s.files["usr/lib/alias"].SHA256!=old||s.files["usr/lib/target"].SHA256!=next{t.Fatal("hardlink was rebound to a later replacement")}
}

func TestAssemblyInventoryPreservesBaseDirectoryAlias(t *testing.T) {
	for _,targetKind:=range []string{"directory","regular"} {
		t.Run(targetKind,func(t *testing.T){
			s:=newAssemblyInputInventory()
			base:=documentinputs.Base{Files:[]documentinputs.File{
				{Path:"usr/lib",Kind:targetKind,Bytes:3,SHA256:assemblyCatalogDigest([]byte("old"))},
				{Path:"lib",Kind:"symlink",Target:"usr/lib"},
			}}
			if err:=s.addBase(base);err!=nil{t.Fatal(err)}
			payload:=assemblyCatalogDigest([]byte("new"))
			err:=s.addDeb(documentinputs.Deb{Package:documentinputs.Package{Name:"sample",Version:"1",Architecture:"amd64"},
				Files:[]documentinputs.File{{Path:"lib",Kind:"directory"},{Path:"lib/sample.so",Kind:"regular",Bytes:3,SHA256:payload}}})
			if targetKind=="regular" {if err==nil{t.Fatal("directory alias to regular file accepted")};return}
			if err!=nil{t.Fatal(err)}
			if s.files["lib"].Kind!="symlink"||s.files["usr/lib/sample.so"].SHA256!=payload{t.Fatal("package directory overwrote base alias")}
			if _,exists:=s.files["lib/sample.so"];exists{t.Fatal("payload escaped the base directory alias")}
		})
	}
}

func TestAssemblyInventoryNoticeMustMatchSelectedPackage(t *testing.T) {
	s:=newAssemblyInputInventory()
	old:=assemblyCatalogDigest([]byte("old grant"));next:=assemblyCatalogDigest([]byte("new grant"))
	base:=documentinputs.Base{Packages:[]documentinputs.Package{{Name:"sample",Version:"1",Architecture:"amd64"}},
		Files:[]documentinputs.File{{Path:"usr/share/doc/sample/copyright",Owner:"sample",Kind:"regular",Bytes:9,SHA256:old}},
		Notices:[]documentinputs.Notice{{Path:"usr/share/doc/sample/copyright",Owner:"sample",Bytes:9,SHA256:old}}}
	if err:=s.addBase(base);err!=nil{t.Fatal(err)}
	if err:=s.addDeb(documentinputs.Deb{Package:documentinputs.Package{Name:"sample",Version:"2",Architecture:"amd64"},
		Files:[]documentinputs.File{{Path:"usr/share/doc/sample/copyright",Kind:"regular",Bytes:9,SHA256:next}},
		Notices:[]documentinputs.Notice{{Path:"usr/share/doc/sample/copyright",Bytes:9,SHA256:next}}});err!=nil{t.Fatal(err)}
	if err:=s.finish(map[string]assemblyDependencyPackage{"sample":{Name:"sample",Version:"2",Architecture:"amd64"}});err!=nil{t.Fatal(err)}
	if s.noticeMatches("debian/sample",old,9)||!s.noticeMatches("debian/sample",next,9){t.Fatal("old package notice survived replacement")}
}

func TestAssemblyInventoryRefusesEscapingOrCyclicCopyright(t *testing.T) {
	for _,target:=range []string{"../../../../../outside","copyright"} {
		t.Run(target,func(t *testing.T){
			s:=newAssemblyInputInventory()
			if err:=s.put(documentinputs.File{Path:"usr/share/doc/sample/copyright",Kind:"symlink",Target:target},"debian/sample","",false);err!=nil{t.Fatal(err)}
			if err:=s.finish(map[string]assemblyDependencyPackage{"sample":{Name:"sample",Version:"1",Architecture:"amd64"}});err==nil{t.Fatal("unsafe notice link accepted")}
		})
	}
}

func TestAssemblyBinaryInventoryMustMatchOriginalControlRelations(t *testing.T) {
	name,version,arch,depends:="sample","1","amd64","libc6 (>= 2.36),zlib1g"
	capture:=&debianBinarySemanticCapture{}
	capture.controls[0]=assemblyDebianBinaryFieldsV1{Package:&name,Version:&version,Architecture:&arch,Depends:&depends}
	value:=assemblyDependencyPackage{Name:name,Version:version,Architecture:arch,Depends:"libc6 (>= 2.36), zlib1g"}
	if err:=compareAssemblyBinaryControl(capture,0,value);err!=nil{t.Fatal(err)}
	value.Depends=""
	if err:=compareAssemblyBinaryControl(capture,0,value);err==nil{t.Fatal("package inventory severed from original dependency control")}
}
