package runtimeoci

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func documentTestHash(data []byte) string { h:=sha256.Sum256(data);return hex.EncodeToString(h[:]) }
func documentTestTar(t *testing.T, entries map[string][]byte) []byte {
	t.Helper();var out bytes.Buffer;w:=tar.NewWriter(&out)
	names:=make([]string,0,len(entries));for name:=range entries{names=append(names,name)};sort.Strings(names)
	for _,name:=range names{data:=entries[name];if err:=w.WriteHeader(&tar.Header{Name:name,Mode:0644,Size:int64(len(data)),Typeflag:tar.TypeReg});err!=nil{t.Fatal(err)};if _,err:=w.Write(data);err!=nil{t.Fatal(err)}}
	if err:=w.Close();err!=nil{t.Fatal(err)};return out.Bytes()
}

func TestDocumentPrebuiltRequiresIndependentPinAndActualExtractorBytes(t *testing.T) {
	tool:=[]byte("print('document extractor protocol 1')\n")
	layer:=documentTestTar(t,map[string][]byte{"opt/luci/document_extractor.py":tool})
	diff:="sha256:"+documentTestHash(layer)
	config:=[]byte(`{"os":"linux","architecture":"amd64","config":{"User":"1001:1001","Entrypoint":["/usr/bin/python3","-I","/opt/luci/document_extractor.py"],"WorkingDir":"/tmp","Labels":{"org.opencontainers.image.revision":"`+strings.Repeat("5",40)+`","luci.document-source":"`+strings.Repeat("6",64)+`","luci.document-source-retention":"`+strings.Repeat("7",64)+`"},"Volumes":{},"ExposedPorts":{}},"rootfs":{"diff_ids":["`+diff+`"]}}`)
	id:="sha256:"+documentTestHash(config)
	configPath:=strings.TrimPrefix(id,"sha256:")+".json"
	index:=[]byte(`[{"Config":"`+configPath+`","RepoTags":[],"Layers":["layer/layer.tar"]}]`)
	parts:=map[string][]byte{configPath:config,"layer/layer.tar":layer,"manifest.json":index}
	archive:=documentTestTar(t,parts)
	var manifest DocumentPackagedManifest
	manifest.Version=1;manifest.Kind="luci-document-prebuilt-docker-save";manifest.ToolProtocol=1
	manifest.Provenance.AssemblyReceiptSHA256=strings.Repeat("1",64);manifest.Provenance.InputClosureSHA256=strings.Repeat("2",64)
	manifest.Provenance.CaptureSHA256=strings.Repeat("3",64);manifest.Provenance.CaptureBytes=1024
	notices:=[]byte(`{"kind":"test-notices-v1"}`)
	manifest.Provenance.NoticeClosureSHA256=documentTestHash(notices);manifest.Provenance.NoticeClosureFileName="document-runtime-notices-v1.json";manifest.Provenance.NoticeClosureBytes=int64(len(notices));manifest.Provenance.SourceCommit=strings.Repeat("5",40)
	manifest.Provenance.SourceDigest=strings.Repeat("6",64);manifest.Provenance.SourceRetentionDigest=strings.Repeat("7",64)
	manifest.Archive.FileName="document-image-linux-amd64.tar";manifest.Archive.Bytes=int64(len(archive));manifest.Archive.SHA256=documentTestHash(archive)
	manifest.Image.ID=id;manifest.Image.ConfigPath=configPath;manifest.Image.LayerPath="layer/layer.tar";manifest.Image.DiffID=diff;manifest.ExtractorSHA256=documentTestHash(tool)
	for name,data:=range parts{manifest.Archive.Members=append(manifest.Archive.Members,Member{Path:name,SHA256:documentTestHash(data),Bytes:int64(len(data))})}
	sort.Slice(manifest.Archive.Members,func(i,j int)bool{return manifest.Archive.Members[i].Path<manifest.Archive.Members[j].Path})
	raw,err:=json.Marshal(manifest);if err!=nil{t.Fatal(err)}
	root:=t.TempDir()
	if err=os.WriteFile(filepath.Join(root,manifest.Archive.FileName),archive,0600);err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(root,"document-runtime-manifest-v1.json"),raw,0600);err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(root,"document-runtime-notices-v1.json"),notices,0600);err!=nil{t.Fatal(err)}
	if _,err=OpenDocument(root,strings.Repeat("a",64));err==nil{t.Fatal("self asserted document manifest was accepted")}
	b,err:=OpenDocument(root,documentTestHash(raw));if err!=nil{t.Fatal(err)};defer b.Close()
	if err=b.VerifyArchive();err!=nil{t.Fatal(err)}
	changed:=append([]byte(nil),raw...);changed[len(changed)-2]='x'
	previous:=b.manifestBytes;b.manifestBytes=changed
	if err=b.VerifyArchive();err==nil{t.Fatal("changed manifest accepted")}
	b.manifestBytes=previous
	manifest.ExtractorSHA256=strings.Repeat("b",64)
	changed,err=json.Marshal(manifest);if err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(root,"document-runtime-manifest-v1.json"),changed,0600);err!=nil{t.Fatal(err)}
	if _,err=OpenDocument(root,documentTestHash(changed));err==nil{t.Fatal("missing actual extractor digest accepted")}
}

func TestDocumentPrebuiltRefusesSandboxAndMalformedMembers(t *testing.T) {
	m:=DocumentPackagedManifest{Version:1,Kind:"luci-runtime-docker-save",ToolProtocol:1}
	if validDocumentManifest(m){t.Fatal("sandbox harness accepted as document image")}
	m.Kind="luci-document-prebuilt-docker-save";m.Archive.FileName="document-image-linux-amd64.tar";m.Archive.SHA256=strings.Repeat("a",64);m.Archive.Bytes=1024
	m.Provenance.AssemblyReceiptSHA256=strings.Repeat("1",64);m.Provenance.InputClosureSHA256=strings.Repeat("2",64)
	m.Provenance.CaptureSHA256=strings.Repeat("3",64);m.Provenance.CaptureBytes=1024
	m.Provenance.NoticeClosureSHA256=strings.Repeat("4",64);m.Provenance.NoticeClosureFileName="document-runtime-notices-v1.json";m.Provenance.NoticeClosureBytes=1;m.Provenance.SourceCommit=strings.Repeat("5",40)
	m.Provenance.SourceDigest=strings.Repeat("6",64);m.Provenance.SourceRetentionDigest=strings.Repeat("7",64)
	m.Image.ID="sha256:"+strings.Repeat("b",64);m.Image.ConfigPath=strings.Repeat("b",64)+".json";m.Image.LayerPath="layer/layer.tar";m.Image.DiffID="sha256:"+strings.Repeat("c",64);m.ExtractorSHA256=strings.Repeat("d",64)
	m.Archive.Members=[]Member{{Path:m.Image.ConfigPath,SHA256:strings.Repeat("b",64),Bytes:1},{Path:m.Image.LayerPath,SHA256:strings.Repeat("c",64),Bytes:1},{Path:"manifest.json",SHA256:strings.Repeat("e",64),Bytes:1}}
	if !validDocumentManifest(m){t.Fatal("valid document inventory refused")}
	m.Archive.Members[1].Path="../layer.tar"
	if validDocumentManifest(m){t.Fatal("unsafe archive path accepted")}
}

func TestDocumentLayerCanonicalDirectoryAndPrefixedNames(t *testing.T) {
	for raw,want := range map[string]string{"usr/":"usr","./opt/luci/document_extractor.py":"opt/luci/document_extractor.py","././opt/luci/":"opt/luci","./usr/lib/libstdc++.so.6":"usr/lib/libstdc++.so.6"} {
		got,ok:=documentLayerName(raw,strings.HasSuffix(raw,"/"))
		if !ok||got!=want{t.Fatalf("ordinary rootfs name %q became %q, %v",raw,got,ok)}
	}
	for _,raw:=range []string{"/opt/luci/document_extractor.py","./../opt/luci/document_extractor.py","opt//luci/document_extractor.py","opt/./luci/document_extractor.py","opt/luci/../document_extractor.py","opt\\luci/document_extractor.py"} {
		if _,ok:=documentLayerName(raw,false);ok{t.Fatalf("unsafe rootfs name accepted: %q",raw)}
	}
	first,_:=documentLayerName("./opt/luci/document_extractor.py",false)
	second,_:=documentLayerName("opt/luci/document_extractor.py",false)
	if first!=second{t.Fatal("equivalent rootfs paths do not share duplicate identity")}
}
