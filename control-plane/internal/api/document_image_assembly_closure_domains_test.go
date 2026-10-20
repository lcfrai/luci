package api

import (
	"reflect"
	"strings"
	"testing"
)

func TestAssemblyDependencyVersionOrdering(t *testing.T) {
	for _,c:=range []struct{a,b string;want int}{
		{"1.0~rc1","1.0",-1},{"1.0","1.0-0",0},{"1:1.0","2.0",1},
		{"1.0-1+b2","1.0-1+b11",-1},{"1.01","1.1",0},
		{"1.0a","1.0+",-1},{"1.0~~","1.0~",-1},
		{"1.99999999999999999999999999999","1.100000000000000000000000000000",-1},
	} {
		t.Run(c.a+"_versus_"+c.b,func(t *testing.T){
			if !validAssemblyDebianVersion(c.a)||!validAssemblyDebianVersion(c.b){t.Fatal("valid version refused")}
			got:=compareAssemblyDebianVersion(c.a,c.b);if got<0{got=-1};if got>0{got=1}
			if got!=c.want{t.Fatal("incorrect Debian ordering",got,c.want)}
			back:=compareAssemblyDebianVersion(c.b,c.a);if got<0&&back<=0||got>0&&back>=0||got==0&&back!=0{t.Fatal("asymmetric ordering")}
		})
	}
}

func TestAssemblyDependencyClosureUsesActualAlternativeAndVersionedProvide(t *testing.T) {
	packages:=[]assemblyDependencyPackage{
		{Name:"consumer",Version:"2.0",Architecture:"amd64",PreDepends:"libc6 (>= 2.36)",Depends:"absent | virtual-api (>= 2.0), interpreter:any"},
		{Name:"provider",Version:"1.0",Architecture:"amd64",Provides:"virtual-api (= 2.1)"},
		{Name:"libc6",Version:"2.36-9+deb12u13",Architecture:"amd64"},
		{Name:"interpreter",Version:"3.11.2",Architecture:"all",MultiArch:"allowed"},
	}
	got,err:=deriveAssemblyDependencyClosure(packages)
	if err!=nil||len(got.Edges)!=3{t.Fatal(got,err)}
	want:=[]assemblyDependencyEdge{{"consumer","pre-depends",0,0,"libc6","2.36-9+deb12u13"},{"consumer","depends",0,1,"provider","2.1"},{"consumer","depends",1,0,"interpreter","3.11.2"}}
	if !reflect.DeepEqual(got.Edges,want){t.Fatal("dependency witness did not bind chosen real provider",got.Edges)}
	if packages[0].Name!="consumer"||got.Packages[1].Name!="interpreter"{t.Fatal("caller mutation or noncanonical ordering")}
	packages[0].Version="99"
	if got.Packages[0].Version!="2.0"{t.Fatal("closure retained mutable caller slice")}
}

func TestAssemblyDependencyClosureRefusesUnclosedOrUnsupportedRelations(t *testing.T) {
	base:=[]assemblyDependencyPackage{{Name:"consumer",Version:"1.0",Architecture:"amd64",Depends:"provider (>= 2)"},{Name:"provider",Version:"2",Architecture:"amd64"}}
	for _,scenario:=range []string{"missing","older","unversioned-provide","conflicts","breaks","duplicate","source-restriction","source-profile","invalid-epoch","bad-provides","malformed-alternative","wrong-any","unsupported-architecture"} {
		t.Run(scenario,func(t *testing.T){
			rows:=append([]assemblyDependencyPackage(nil),base...)
			switch scenario {
			case "missing":rows=rows[:1]
			case "older":rows[1].Version="1.9"
			case "unversioned-provide":rows[1].Name="implementation";rows[1].Provides="provider"
			case "conflicts":rows[0].Conflicts="provider"
			case "breaks":rows[0].Breaks="provider (<< 3)"
			case "duplicate":rows=append(rows,rows[1])
			case "source-restriction":rows[0].Depends="provider [amd64]"
			case "source-profile":rows[0].Depends="provider <!nocheck>"
			case "invalid-epoch":rows[1].Version="x:2"
			case "bad-provides":rows[1].Provides="virtual (>= 2)"
			case "malformed-alternative":rows[0].Depends="provider |"
			case "wrong-any":rows[0].Depends="provider:any"
			case "unsupported-architecture":rows[1].Architecture="arm64"
			}
			if got,err:=deriveAssemblyDependencyClosure(rows);err==nil||got.Packages!=nil||got.Edges!=nil{t.Fatal("incomplete domain acquired a closure",got,err)}
		})
	}
}

func TestAssemblyDependencyParserHasFiniteBoundsAndNoIgnoredSyntax(t *testing.T) {
	for _,value:=range []string{"provider\n", "provider\x00",strings.Repeat("a",65537),strings.Repeat("a,",513),strings.Repeat("a | ",33)+"a","provider (= )","provider (== 1)"} {
		if _,err:=parseAssemblyRelations(value);err==nil{t.Fatal("unsupported relation accepted")}
	}
	if _,err:=deriveAssemblyDependencyClosure(make([]assemblyDependencyPackage,513));err==nil{t.Fatal("unbounded universe accepted")}
	if _,err:=assemblyDependencyFromControl(assemblyDebianBinaryFieldsV1{});err==nil{t.Fatal("missing observed package identity accepted")}
}
