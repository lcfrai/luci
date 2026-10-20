package dockerbe

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func domainTestInput(role,path string,bytes int64)backend.DomainInput{
	digest:=sha256.Sum256(nil)
	return backend.DomainInput{Role:role,RelativePath:path,Bytes:bytes,SHA256:hex.EncodeToString(digest[:])}
}

func TestDocumentDomainInputMetadataDemand(t *testing.T){
	root:=filepath.Join(t.TempDir(),"declared") // path metadata only; no input IO
	inputs:=[]backend.DomainInput{domainTestInput("base","base/image.tar",123),domainTestInput("wheel","wheels/package.whl",456)}
	demand,err:=PlanDocumentDomainInputs(root,inputs)
	if err!=nil{t.Fatal(err)}
	if demand.Inputs!=2||demand.DeclaredBytes!=579||demand.OriginalHandles<6||demand.OriginalHandles>backend.DocumentDomainInputHandleLimit||demand.BlockBytes!=64<<10||demand.ReadCalls!=1048576||demand.ReadBytes!=64<<30{t.Fatalf("unexpected finite demand: %+v",demand)}
	p,err:=planDocumentDomainInputs(root,inputs);if err!=nil{t.Fatal(err)}
	inputs[0].Role="mutated"
	if p.inputs[0].Role!="base"{t.Fatal("plan aliases caller metadata")}
}

func TestDocumentDomainSourceRetentionDoesNotAuthorizeLargeReadBuffers(t *testing.T){
	root:=filepath.Join(t.TempDir(),"root")
	inputs:=[]backend.DomainInput{domainTestInput("chromium","source/chromium.tar.xz",8<<30),domainTestInput("pdfium","source/pdfium.tar.xz",7<<30)}
	demand,err:=PlanDocumentDomainInputs(root,inputs)
	if err!=nil||demand.DeclaredBytes!=15<<30||demand.ReadBytes!=64<<30||demand.ReadCalls!=1048576||demand.ReadBufferBytes!=2*(64<<10){t.Fatalf("large original streaming demand unavailable: %+v %v",demand,err)}
	// Numeric request validation, no multi-gigabyte fixture or allocation.
	if !validDomainReadAtBytes(128<<20)||validDomainReadAtBytes((128<<20)+1)||validDomainReadAtBytes(8<<30)||validDomainReadAtBytes(-1){t.Fatal("source-retention envelope leaked into caller-buffer authority")}
	inputs=append(inputs,domainTestInput("third","source/third.tar.xz",2<<30))
	if _,err=PlanDocumentDomainInputs(root,inputs);err==nil{t.Fatal("16GiB unique retained-input ceiling widened")}
}

func TestDocumentDomainInputMetadataRefusesAmbiguityAndEnvelope(t *testing.T){
	root:=filepath.Join(t.TempDir(),"root")
	valid:=domainTestInput("source","sources/original.tgz",1)
	for _,name:=range []string{"../outside","/absolute","a\\b","a:b","a/../b","a//b","NUL.txt","a./b","a /b","a\x00b","COM1.bin"}{
		t.Run("path="+name,func(t *testing.T){v:=valid;v.RelativePath=name;if _,err:=PlanDocumentDomainInputs(root,[]backend.DomainInput{v});err==nil{t.Fatal("unsafe path admitted")}})
	}
	for _,test:=range []struct{name string;inputs []backend.DomainInput}{
		{"duplicate-role",[]backend.DomainInput{valid,{Role:"source",RelativePath:"other",Bytes:1,SHA256:valid.SHA256}}},
		{"case-alias",[]backend.DomainInput{valid,{Role:"other",RelativePath:"SOURCES/original.tgz",Bytes:1,SHA256:valid.SHA256}}},
		{"negative",[]backend.DomainInput{{Role:"source",RelativePath:"file",Bytes:-1,SHA256:valid.SHA256}}},
		{"file-cap",[]backend.DomainInput{{Role:"source",RelativePath:"file",Bytes:backend.DocumentDomainInputFileBytes+1,SHA256:valid.SHA256}}},
		{"bad-digest",[]backend.DomainInput{{Role:"source",RelativePath:"file",Bytes:1,SHA256:"not-a-digest"}}},
	}{t.Run(test.name,func(t *testing.T){if _,err:=PlanDocumentDomainInputs(root,test.inputs);err==nil{t.Fatal("invalid envelope admitted")}})}
	inputs:=make([]backend.DomainInput,5);for i:=range inputs{inputs[i]=domainTestInput(string(rune('a'+i)),string(rune('a'+i)),backend.DocumentDomainInputFileBytes)}
	if _,err:=PlanDocumentDomainInputs(root,inputs);err==nil{t.Fatal("aggregate cap widened")}
	if owner,err:=OpenDocumentDomainInputs(context.Background(),root,[]backend.DomainInput{valid});err==nil||owner!=nil{t.Fatal("missing caller deadline admitted")}
}
