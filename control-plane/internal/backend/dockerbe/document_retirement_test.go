package dockerbe

import (
 "testing"
 "github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestHistoricalDocumentRetirementDoesNotExposeNewCompute(t *testing.T){
 if got,err:=NewDocumentRetirement(nil);err==nil||got!=nil{t.Fatal("missing selected backend became historical ownership")}
 // Constructor metadata only: no engine call, package, fake image or parser.
 got,err:=NewDocumentRetirement(&Backend{});if err!=nil||got==nil{t.Fatal(err)}
 if _,compute:=got.(backend.DocumentExtractionExecutor);compute{t.Fatal("retirement-only capability exposed prepare/create/start/attach")}
}
