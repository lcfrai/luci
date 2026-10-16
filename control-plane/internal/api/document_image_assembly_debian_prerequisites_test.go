package api

import (
	"strings"
	"testing"
	"unicode/utf16"
)

// Pure metadata only. These values do not assert SHA preimages, a selected
// product package, authenticated modules or an actually supported current host.
func syntheticDebianHostRequirements(guest bool) debianHostRequirements {
	r:=debianHostRequirements{minimumBuild:19041,i386Required:guest,toolDomainDigest:strings.Repeat("a",64),moduleCount:1}
	r.modules[0]="ucrtbase.dll";return r
}

func syntheticDebianHostDirectory(b *[debianHostDirectoryUnits]uint16,s string)uint32{
	v:=utf16.Encode([]rune(s));copy(b[:],v);b[len(v)]=0;return uint32(len(v))
}

func TestDebianHostRequirementsAreDeclaredAndCopied(t *testing.T){
	d:=&assemblyDebianToolDomainV1{digest:strings.Repeat("a",64),record:assemblyToolDomainRecordV1{SystemABI:assemblyToolSystemABIV1{OS:"windows",HostArchitecture:"amd64",MinimumWindowsBuild:19041,CRTMode:"supported-system-ucrt",RequiresWOW64:true,SystemModules:[]string{"ucrtbase.dll"}}}}
	r,e:=debianHostRequirementsFromDomain(d);if e!=nil{t.Fatal(e)}
	d.record.SystemABI.SystemModules[0]="changed.dll"
	if r.modules[0]!="ucrtbase.dll"||!r.i386Required||r.minimumBuild!=19041{t.Fatal("requirement alias or field loss")}
	d.record.SystemABI.MinimumWindowsBuild=0
	if _,e=debianHostRequirementsFromDomain(d);e==nil{t.Fatal("unset selected minimum accepted")}
	if _,e=debianHostRequirementsFromDomain(nil);e==nil{t.Fatal("missing declared domain accepted")}
}

func TestDebianHostDirectoryFramingRefusesInvalidInputs(t *testing.T){
	for _,name:=range []string{"zero","full","no-terminator","embedded-nul","high-surrogate","low-surrogate","relative","traversal","unc","slash","trailing-dot"}{
		t.Run(name,func(t *testing.T){
			var b [debianHostDirectoryUnits]uint16;n:=syntheticDebianHostDirectory(&b,`C:\Windows\System32`)
			switch name{
			case "zero":n=0
			case "full":n=debianHostDirectoryUnits
			case "no-terminator":b[n]='x'
			case "embedded-nul":b[4]=0
			case "high-surrogate":b[n-1]=0xd800
			case "low-surrogate":b[4]=0xdc00
			case "relative":n=syntheticDebianHostDirectory(&b,`Windows\System32`)
			case "traversal":n=syntheticDebianHostDirectory(&b,`C:\Windows\..\System32`)
			case "unc":n=syntheticDebianHostDirectory(&b,`\\server\share`)
			case "slash":n=syntheticDebianHostDirectory(&b,`C:\Windows/System32`)
			case "trailing-dot":n=syntheticDebianHostDirectory(&b,`C:\Windows.\System32`)
			}
			if _,e:=decodeDebianHostDirectory(&b,n);e==nil{t.Fatal("invalid framed directory accepted")}
		})
	}
	var b [debianHostDirectoryUnits]uint16;n:=syntheticDebianHostDirectory(&b,`D:\Windows\System32`)
	if v,e:=decodeDebianHostDirectory(&b,n);e!=nil||v!=`D:\Windows\System32`{t.Fatalf("valid local directory: %q %v",v,e)}
}

func TestDebianHostFactsDoNotAcceptMissingOrMismatchedRequirements(t *testing.T){
	r:=syntheticDebianHostRequirements(false)
	f:=debianHostFacts{nativeMachine:debianHostMachineAMD64,major:10,build:19041,platform:2,nativeDirectory:`C:\Windows\System32`,toolDomainDigest:r.toolDomainDigest}
	if e:=validateDebianHostFacts(r,f);e!=nil{t.Fatal(e)}
	for _,name:=range []string{"unset-minimum","foreign-domain","arm64-native","emulated-process","old-build","old-os","non-nt","guest-unexpected","guest-required"}{
		t.Run(name,func(t *testing.T){a,b:=r,f;switch name{
		case "unset-minimum":a.minimumBuild=0
		case "foreign-domain":b.toolDomainDigest=strings.Repeat("b",64)
		case "arm64-native":b.nativeMachine=0xaa64
		case "emulated-process":b.processMachine=debianHostMachineAMD64
		case "old-build":b.build=19040
		case "old-os":b.major=6
		case "non-nt":b.platform=1
		case "guest-unexpected":b.i386Queried=true
		case "guest-required":a.i386Required=true
		};if validateDebianHostFacts(a,b)==nil{t.Fatal("incompatible observations accepted")}})
	}
}
