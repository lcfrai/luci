package documentinputs

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"fmt"
	"strings"
	"testing"

	"github.com/ulikunitz/xz"
)

func fixturePin(raw []byte) Pin { return Pin{SHA256:hashBytes(raw),Bytes:int64(len(raw))} }
func fixtureTar(t *testing.T,entries map[string][]byte) []byte {
	t.Helper();var out bytes.Buffer;writer:=tar.NewWriter(&out)
	for name,raw:=range entries{if e:=writer.WriteHeader(&tar.Header{Name:name,Mode:0444,Size:int64(len(raw))});e!=nil{t.Fatal(e)};if _,e:=writer.Write(raw);e!=nil{t.Fatal(e)}}
	if e:=writer.Close();e!=nil{t.Fatal(e)};return out.Bytes()
}
func fixtureGzip(t *testing.T,raw []byte) []byte {
	t.Helper();var out bytes.Buffer;writer:=gzip.NewWriter(&out)
	if _,e:=writer.Write(raw);e!=nil{t.Fatal(e)};if e:=writer.Close();e!=nil{t.Fatal(e)};return out.Bytes()
}
func TestWheelRECORDAndActualNoticeBytes(t *testing.T) {
	owner:=WheelOwner{ID:"sample",Version:"1.0",InstalledOwner:"sample-1.0.dist-info"}
	files:=map[string][]byte{
		"sample/__init__.py":[]byte("value=1\n"),
		"sample-1.0.dist-info/METADATA":[]byte("Metadata-Version: 2.4\nName: sample\nVersion: 1.0\nRequires-Dist: alpha >= 1\nRequires-Dist: beta >= 2\nRequires-Python: >=3.10\nClassifier: Topic :: Utilities\nClassifier: Programming Language :: Python\n\nA wheel description:\nwith non-header lines and ünicode.\n"),
		"sample-1.0.dist-info/WHEEL":[]byte("Wheel-Version: 1.0\nGenerator: source-fixture\nRoot-Is-Purelib: true\nTag: py3-none-any\n"),
		"sample-1.0.dist-info/LICENSE":[]byte("The original grant\n"),
	}
	var record bytes.Buffer;csvWriter:=csv.NewWriter(&record)
	for name,raw:=range files{
		sum:=sha256.Sum256(raw)
		if e:=csvWriter.Write([]string{name,"sha256="+base64.RawURLEncoding.EncodeToString(sum[:]),fmt.Sprint(len(raw))});e!=nil{t.Fatal(e)}
	}
	if e:=csvWriter.Write([]string{"sample-1.0.dist-info/RECORD","",""});e!=nil{t.Fatal(e)}
	csvWriter.Flush();if csvWriter.Error()!=nil{t.Fatal(csvWriter.Error())}
	files["sample-1.0.dist-info/RECORD"]=record.Bytes()
	encode:=func()[]byte{var out bytes.Buffer;writer:=zip.NewWriter(&out);for name,raw:=range files{entry,e:=writer.Create(name);if e!=nil{t.Fatal(e)};if _,e=entry.Write(raw);e!=nil{t.Fatal(e)}};if e:=writer.Close();e!=nil{t.Fatal(e)};return out.Bytes()}
	wheel:=encode()
	observed,e:=InspectWheel(context.Background(),bytes.NewReader(wheel),int64(len(wheel)),fixturePin(wheel),owner)
	if e!=nil||len(observed.Files)!=5||len(observed.Notices)!=1||string(observed.Notices[0].Contents)!="The original grant\n"||len(observed.RequiresDist)!=2||observed.RequiresPython!=">=3.10"||len(observed.Tags)!=1||observed.Tags[0]!="py3-none-any"||!observed.RootIsPurelib{t.Fatalf("actual wheel inventory unavailable: %v",e)}
	files["sample/__init__.py"]=[]byte("changed\n")
	changed:=encode()
	if _,e=InspectWheel(context.Background(),bytes.NewReader(changed),int64(len(changed)),fixturePin(changed),owner);e==nil{t.Fatal("RECORD mismatch accepted")}
}

func TestParagraphFieldsAcceptsDebianIdentityAndSourceChecksumKeys(t *testing.T) {
	rows,e:=paragraphFields([]byte("Package: sample\nName: displayed\nChecksums-Sha256:\n abc123 7 archive.tar.gz\n\n"),1<<20)
	if e!=nil||len(rows)!=1||rows[0]["Package"]!="sample"||rows[0]["Name"]!="displayed"||rows[0]["Checksums-Sha256"]!="\nabc123 7 archive.tar.gz"{t.Fatalf("ordinary control keys unavailable: %v",e)}
}

func TestWheelMetadataRepeatedHeadersAndDescription(t *testing.T) {
	raw:=[]byte("Metadata-Version: 2.4\nName: sample\nVersion: 1.0\nRequires-Dist: alpha\nRequires-Dist: beta\nClassifier: one\nClassifier: two\n\nDescription: this is body text, not a header\n")
	identity,e:=parseWheelMetadata(raw)
	if e!=nil||identity.name!="sample"||identity.version!="1.0"||len(identity.requiresDist)!=2{t.Fatalf("valid wheel metadata unavailable: %v",e)}
	if _,e=parseWheelMetadata([]byte("Name: sample\nName: other\nVersion: 1.0\n"));e==nil{t.Fatal("ambiguous wheel identity accepted")}
}

func TestWheelDirectoryPreflightRefusesUnboundedEntryCount(t *testing.T) {
	record:=make([]byte,22)
	copy(record,[]byte("PK\x05\x06"))
	record[8]=0xff;record[9]=0xff;record[10]=0xff;record[11]=0xff
	if preflightWheelDirectory(bytes.NewReader(record),int64(len(record)))==nil{t.Fatal("unbounded central-directory count accepted")}
}

func TestELFMagicAccumulatesAcrossShortReads(t *testing.T) {
	var capture elfCapture
	for _,part:=range [][]byte{{0x7f},{'E'},{'L'},{'F',1,2}} {
		if capture.write(part)!=nil{t.Fatal("short ELF prefix rejected")}
	}
	if !capture.isELF||!bytes.Equal(capture.raw.Bytes(),[]byte{0x7f,'E','L','F',1,2}){t.Fatal("short reads lost native magic or bytes")}
}

func TestBaseStatusFilesAndCopyrightAreObserved(t *testing.T){
	layer:=fixtureTar(t,map[string][]byte{
		"var/lib/dpkg/status":[]byte("Package: sample\nVersion: 1.0\nArchitecture: amd64\nStatus: install ok installed\nSource: sample-src (1.0)\nBuilt-Using: helper (= 2)\n\n"),
		"var/lib/dpkg/info/sample.list":[]byte("/usr/bin/sample\n/usr/share/doc/sample/copyright\n"),
		"usr/bin/sample":[]byte("#!/bin/sh\nexit 0\n"),
		"usr/share/doc/sample/copyright":[]byte("Original copyright grant\n"),
	})
	compressed:=fixtureGzip(t,layer)
	observed,e:=InspectBase(context.Background(),bytes.NewReader(compressed),fixturePin(compressed))
	if e!=nil||len(observed.Packages)!=1||observed.Packages[0].Source!="sample-src (1.0)"||len(observed.Notices)!=1||string(observed.Notices[0].Contents)!="Original copyright grant\n"{t.Fatalf("actual base inventory unavailable: %v",e)}
	if observed.Notices[0].Owner!="sample"{t.Fatal("dpkg .list owner was not joined")}
}

func fixtureAR(t *testing.T,entries [][2][]byte) []byte {
	t.Helper();var out bytes.Buffer;out.WriteString("!<arch>\n")
	for _,entry:=range entries{
		name,data:=string(entry[0]),entry[1]
		header:=fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n",name+"/",0,0,0,0644,len(data))
		if len(header)!=60{t.Fatalf("bad ar fixture header %d",len(header))}
		out.WriteString(header);out.Write(data);if len(data)%2==1{out.WriteByte('\n')}
	}
	return out.Bytes()
}

func TestDebControlTupleAndRealDataInventory(t *testing.T){
	control:=fixtureGzip(t,fixtureTar(t,map[string][]byte{"control":[]byte("Package: sample\nVersion: 1.0\nArchitecture: amd64\nMulti-Arch: same\nDepends: libc6 (>= 2)\n\n"),"conffiles":[]byte("/etc/sample.conf\n")}))
	data:=fixtureGzip(t,fixtureTar(t,map[string][]byte{"usr/bin/sample":[]byte("binary content\n"),"etc/sample.conf":[]byte("setting=yes\n"),"usr/share/doc/sample/copyright":[]byte("Debian grant bytes\n")}))
	deb:=fixtureAR(t,[][2][]byte{{[]byte("debian-binary"),[]byte("2.0\n")},{[]byte("control.tar.gz"),control},{[]byte("data.tar.gz"),data}})
	observed,e:=InspectDeb(context.Background(),bytes.NewReader(deb),int64(len(deb)),fixturePin(deb),DebExpectation{Name:"sample",Version:"1.0",Architecture:"amd64"})
	if e!=nil||observed.Package.MultiArch!="same"||len(observed.Files)!=3||len(observed.Conffiles)!=1||observed.Conffiles[0]!="/etc/sample.conf"||len(observed.Notices)!=1||string(observed.Notices[0].Contents)!="Debian grant bytes\n"{t.Fatalf("actual deb inventory unavailable: %v",e)}
	if _,e=InspectDeb(context.Background(),bytes.NewReader(deb),int64(len(deb)),fixturePin(deb),DebExpectation{Name:"other",Version:"1.0",Architecture:"amd64"});e==nil{t.Fatal("different expected package accepted")}
	missing:=fixtureGzip(t,fixtureTar(t,map[string][]byte{"usr/bin/sample":[]byte("binary content\n")}))
	withoutConffile:=fixtureAR(t,[][2][]byte{{[]byte("debian-binary"),[]byte("2.0\n")},{[]byte("control.tar.gz"),control},{[]byte("data.tar.gz"),missing}})
	if _,e=InspectDeb(context.Background(),bytes.NewReader(withoutConffile),int64(len(withoutConffile)),fixturePin(withoutConffile),DebExpectation{Name:"sample",Version:"1.0",Architecture:"amd64"});e==nil{t.Fatal("declared conffile absent from data tar accepted")}
}

func TestSignedSourcesBindingAndRetainedArchiveBytes(t *testing.T){
	archive:=[]byte("original selected source bytes\n")
	index:=[]byte(fmt.Sprintf("Package: sample-src\nVersion: 1.0\nDirectory: pool/main/s/sample\nChecksums-Sha256:\n %s %d sample_1.0.orig.tar.gz\n\n",hashBytes(archive),len(archive)))
	var compressed bytes.Buffer;writer,e:=xz.NewWriter(&compressed);if e!=nil{t.Fatal(e)}
	if _,e=writer.Write(index);e!=nil{t.Fatal(e)};if e=writer.Close();e!=nil{t.Fatal(e)}
	binding:=SourcesBinding{SignedReleaseSHA256:strings.Repeat("a",64),IndexPath:"main/source/Sources.xz",CompressedIndex:fixturePin(compressed.Bytes()),Cleartext:fixturePin(index)}
	inputs:=[]SourceArchiveInput{{SourceName:"sample-src",SourceVersion:"1.0",FileName:"pool/main/s/sample/sample_1.0.orig.tar.gz",Pin:fixturePin(archive),Reader:bytes.NewReader(archive)}}
	observed,e:=InspectSources(context.Background(),bytes.NewReader(compressed.Bytes()),binding,inputs)
	if e!=nil||len(observed)!=1||len(observed[0].Archives)!=1||observed[0].Archives[0].SHA256!=hashBytes(archive){t.Fatalf("selected Sources paragraph unavailable: %v",e)}
	changed:=append([]byte(nil),archive...);changed[0]^=1
	inputs[0].Reader=bytes.NewReader(changed)
	if _,e=InspectSources(context.Background(),bytes.NewReader(compressed.Bytes()),binding,inputs);e==nil{t.Fatal("changed original source archive accepted")}
}

func TestSourcesRequireEverySelectedChecksumMember(t *testing.T) {
	first:=[]byte("first source archive")
	second:=[]byte("second source archive")
	index:=[]byte(fmt.Sprintf("Package: sample-src\nVersion: 1.0\nDirectory: pool/main/s/sample\nChecksums-Sha256:\n %s %d sample_1.0.dsc\n %s %d sample_1.0.orig.tar.gz\n\n",hashBytes(first),len(first),hashBytes(second),len(second)))
	var compressed bytes.Buffer;writer,e:=xz.NewWriter(&compressed);if e!=nil{t.Fatal(e)}
	if _,e=writer.Write(index);e!=nil{t.Fatal(e)};if e=writer.Close();e!=nil{t.Fatal(e)}
	binding:=SourcesBinding{SignedReleaseSHA256:strings.Repeat("a",64),IndexPath:"main/source/Sources.xz",CompressedIndex:fixturePin(compressed.Bytes()),Cleartext:fixturePin(index)}
	inputs:=[]SourceArchiveInput{{SourceName:"sample-src",SourceVersion:"1.0",FileName:"pool/main/s/sample/sample_1.0.dsc",Pin:fixturePin(first),Reader:bytes.NewReader(first)}}
	if _,e=InspectSources(context.Background(),bytes.NewReader(compressed.Bytes()),binding,inputs);e==nil{t.Fatal("subset of selected source archives accepted")}
	inputs=append(inputs,SourceArchiveInput{SourceName:"sample-src",SourceVersion:"1.0",FileName:"pool/main/s/sample/sample_1.0.orig.tar.gz",Pin:fixturePin(second),Reader:bytes.NewReader(second)})
	selected,e:=InspectSources(context.Background(),bytes.NewReader(compressed.Bytes()),binding,inputs)
	if e!=nil||len(selected)!=1||len(selected[0].Archives)!=2{t.Fatalf("complete selected source unavailable: %v",e)}
}
