package dockerbe

import (
 "context"
 "testing"

 "github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestBinaryMetadataSnapshotRejectsMissingOrForeignTokens(t *testing.T){
 for _,kind:=range []string{"nil-context","missing-tokens","cancelled"}{t.Run(kind,func(t *testing.T){
  ctx:=context.Background();if kind=="nil-context"{ctx=nil};if kind=="cancelled"{var cancel context.CancelFunc;ctx,cancel=context.WithCancel(ctx);cancel()}
  got,err:=SnapshotDocumentImageAssemblyBinaryOriginals(ctx,nil,nil,nil)
  if err==nil||got!=([180]backend.DocumentImageAssemblyBinaryFingerprint{}){t.Fatal("missing originals became a native identity snapshot")}
 })}
}
