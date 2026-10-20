package dockerbe

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

func documentRestoreHash(raw []byte) string { sum:=sha256.Sum256(raw);return hex.EncodeToString(sum[:]) }
func documentRestoreTar(t *testing.T, entries map[string][]byte) []byte {
	t.Helper();var out bytes.Buffer;writer:=tar.NewWriter(&out)
	names:=make([]string,0,len(entries));for name:=range entries{names=append(names,name)};sort.Strings(names)
	for _,name:=range names{
		raw:=entries[name]
		if err:=writer.WriteHeader(&tar.Header{Name:name,Mode:0444,Size:int64(len(raw)),Typeflag:tar.TypeReg});err!=nil{t.Fatal(err)}
		if _,err:=writer.Write(raw);err!=nil{t.Fatal(err)}
	}
	if err:=writer.Close();err!=nil{t.Fatal(err)}
	return out.Bytes()
}

func TestDocumentRestoreRequiresCompletedReceiptAndNeverImportsOrTags(t *testing.T) {
	selected,probe,_:=newRuntimeImageProbe(t)
	tool:=[]byte("print('document protocol 1')\n")
	layer:=documentRestoreTar(t,map[string][]byte{"opt/luci/document_extractor.py":tool})
	diff:="sha256:"+documentRestoreHash(layer)
	config:=[]byte(`{"os":"linux","architecture":"amd64","config":{"User":"1001:1001","Entrypoint":["/usr/bin/python3","-I","/opt/luci/document_extractor.py"],"WorkingDir":"/tmp","Labels":{"org.opencontainers.image.revision":"`+strings.Repeat("5",40)+`","luci.document-source":"`+strings.Repeat("6",64)+`","luci.document-source-retention":"`+strings.Repeat("7",64)+`"}},"rootfs":{"diff_ids":["`+diff+`"]}}`)
	id:="sha256:"+documentRestoreHash(config)
	configPath:=strings.TrimPrefix(id,"sha256:")+".json"
	saved:=[]byte(`[{"Config":"`+configPath+`","RepoTags":[],"Layers":["layer/layer.tar"]}]`)
	parts:=map[string][]byte{configPath:config,"layer/layer.tar":layer,"manifest.json":saved}
	archive:=documentRestoreTar(t,parts)
	var manifest runtimeoci.DocumentPackagedManifest
	manifest.Version=1;manifest.Kind="luci-document-prebuilt-docker-save";manifest.ToolProtocol=1
	manifest.Provenance.AssemblyReceiptSHA256=strings.Repeat("1",64);manifest.Provenance.InputClosureSHA256=strings.Repeat("2",64)
	manifest.Provenance.CaptureSHA256=strings.Repeat("3",64);manifest.Provenance.CaptureBytes=1024
	notices:=[]byte(`{"kind":"test-notices-v1"}`)
	manifest.Provenance.NoticeClosureSHA256=documentRestoreHash(notices);manifest.Provenance.NoticeClosureFileName="document-runtime-notices-v1.json";manifest.Provenance.NoticeClosureBytes=int64(len(notices));manifest.Provenance.SourceCommit=strings.Repeat("5",40)
	manifest.Provenance.SourceDigest=strings.Repeat("6",64);manifest.Provenance.SourceRetentionDigest=strings.Repeat("7",64)
	manifest.Archive.FileName="document-image-linux-amd64.tar";manifest.Archive.SHA256=documentRestoreHash(archive);manifest.Archive.Bytes=int64(len(archive))
	manifest.Image.ID=id;manifest.Image.ConfigPath=configPath;manifest.Image.LayerPath="layer/layer.tar";manifest.Image.DiffID=diff;manifest.ExtractorSHA256=documentRestoreHash(tool)
	for name,raw:=range parts{manifest.Archive.Members=append(manifest.Archive.Members,runtimeoci.Member{Path:name,SHA256:documentRestoreHash(raw),Bytes:int64(len(raw))})}
	sort.Slice(manifest.Archive.Members,func(i,j int)bool{return manifest.Archive.Members[i].Path<manifest.Archive.Members[j].Path})
	raw,err:=json.Marshal(manifest);if err!=nil{t.Fatal(err)}
	directory:=t.TempDir()
	if err=os.WriteFile(filepath.Join(directory,manifest.Archive.FileName),archive,0600);err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(directory,"document-runtime-manifest-v1.json"),raw,0600);err!=nil{t.Fatal(err)}
	if err=os.WriteFile(filepath.Join(directory,"document-runtime-notices-v1.json"),notices,0600);err!=nil{t.Fatal(err)}
	pin:=documentRestoreHash(raw)
	installer,err:=NewDocumentRuntimeInstaller(selected,pin,DocumentAttachConfiguration{Headers:map[string]string{}});if err!=nil{t.Fatal(err)}
	receipt:=backend.DocumentRuntimePublishedReceipt{
		Identity:backend.RuntimeImageIdentity{Protocol:backend.RuntimeImageExecutionProtocol,EngineID:"check-engine",InstanceID:"check-instance",AllocationID:"document-allocation-one",JobID:"document-job-one",ScopeDigest:strings.Repeat("a",64),SourceDigest:pin},
		ImageID:id,ImageTag:"docker.io/luci/document:v1",ManifestSHA256:pin,
	}
	if _,err=installer.RestoreDocumentExtractor(context.Background(),directory,backend.DocumentRuntimePublishedReceipt{});err==nil{t.Fatal("empty historical publication accepted")}
	row:=probe.checkProbe.image;row.ID=id;row.Architecture="amd64"
	probe.images[id],probe.images[receipt.ImageTag]=row,row
	extractor,err:=installer.RestoreDocumentExtractor(context.Background(),directory,receipt)
	if err!=nil||extractor.DocumentImageID()!=id{t.Fatalf("finished exact publication could not restore: %v",err)}
	if len(probe.mutations)!=0||probe.loadCalls!=0{t.Fatal("restart readback repeated installer effects")}
	delete(probe.images,receipt.ImageTag)
	if _,err=installer.RestoreDocumentExtractor(context.Background(),directory,receipt);err==nil{t.Fatal("missing tag accepted from historical receipt")}
	probe.images[receipt.ImageTag]=row
	changed:=receipt;changed.Identity.SourceDigest=strings.Repeat("b",64)
	if _,err=installer.RestoreDocumentExtractor(context.Background(),directory,changed);err==nil{t.Fatal("different source identity accepted")}
}
