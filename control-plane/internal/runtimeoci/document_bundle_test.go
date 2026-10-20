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
	config:=[]byte(`{"os":"linux","architecture":"amd64","config":{"User":"1000:1000","Entrypoint":[],"Volumes":{},"ExposedPorts":{}},"rootfs":{"diff_ids":["`+diff+`"]}}`)
	id:="sha256:"+documentTestHash(config)
	configPath:=strings.TrimPrefix(id,"sha256:")+".json"
	index:=[]byte(`[{"Config":"`+configPath+`","RepoTags":[],"Layers":["layer/layer.tar"]}]`)
	parts:=map[string][]byte{configPath:config,"layer/layer.tar":layer,"manifest.json":index}
	archive:=documentTestTar(t,parts)
	var manifest DocumentPackagedManifest
	manifest.Version=1;manifest.Kind="luci-document-prebuilt-docker-save";manifest.ToolProtocol=1
	manifest.Archive.FileName="document-image-linux-amd64.tar";manifest.Archive.Bytes=int64(len(archive));manifest.Archive.SHA256=documentTestHash(archive)
	manifest.Image.ID=id;manifest.Image.ConfigPath=configPath;manifest.Image.LayerPath="layer/layer.tar";manifest.Image.DiffID=diff;manifest.ExtractorSHA256=documentTestHash(tool)
	for name,data:=range parts{manifest.Archive.Members=append(manifest.Archive.Members,Member{Path:name,SHA256:documentTestHash(data),Bytes:int64(len(data))})}
	sort.Slice(manifest.Archive.Members,func(i,j int)bool{return manifest.Archive.Members[i].Path<manifest.Archive.Members[j].Path})
	raw,err:=json.Marshal(manifest);if err!=nil{t.Fatal(err)}
	root:=t.TempDir()
	if err=os.WriteFile(filepath.Join(root,manifest.Archive.FileName),archive,0600);err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(root,"document-runtime-manifest-v1.json"),raw,0600);err!=nil{t.Fatal(err)}
	if _,_,err=OpenDocument(root,strings.Repeat("a",64));err==nil{t.Fatal("self asserted document manifest was accepted")}
	b,held,err:=OpenDocument(root,documentTestHash(raw));if err!=nil{t.Fatal(err)};defer b.Close()
	if err=VerifyDocumentArchive(b,held);err!=nil{t.Fatal(err)}
	changed:=append([]byte(nil),raw...);changed[len(changed)-2]='x'
	if err=VerifyDocumentArchive(b,changed);err==nil{t.Fatal("changed manifest accepted")}
	manifest.ExtractorSHA256=strings.Repeat("b",64)
	changed,err=json.Marshal(manifest);if err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(root,"document-runtime-manifest-v1.json"),changed,0600);err!=nil{t.Fatal(err)}
	if _,_,err=OpenDocument(root,documentTestHash(changed));err==nil{t.Fatal("missing actual extractor digest accepted")}
}

func TestDocumentPrebuiltRefusesSandboxAndMalformedMembers(t *testing.T) {
	m:=DocumentPackagedManifest{Version:1,Kind:"luci-runtime-docker-save",ToolProtocol:1}
	if validDocumentManifest(m){t.Fatal("sandbox harness accepted as document image")}
	m.Kind="luci-document-prebuilt-docker-save";m.Archive.FileName="document-image-linux-amd64.tar";m.Archive.SHA256=strings.Repeat("a",64);m.Archive.Bytes=1024
	m.Image.ID="sha256:"+strings.Repeat("b",64);m.Image.ConfigPath=strings.Repeat("b",64)+".json";m.Image.LayerPath="layer/layer.tar";m.Image.DiffID="sha256:"+strings.Repeat("c",64);m.ExtractorSHA256=strings.Repeat("d",64)
	m.Archive.Members=[]Member{{Path:m.Image.ConfigPath,SHA256:strings.Repeat("b",64),Bytes:1},{Path:m.Image.LayerPath,SHA256:strings.Repeat("c",64),Bytes:1},{Path:"manifest.json",SHA256:strings.Repeat("e",64),Bytes:1}}
	if !validDocumentManifest(m){t.Fatal("valid document inventory refused")}
	m.Archive.Members[1].Path="../layer.tar"
	if validDocumentManifest(m){t.Fatal("unsafe archive path accepted")}
}
