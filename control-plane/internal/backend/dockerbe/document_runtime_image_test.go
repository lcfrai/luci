package dockerbe

import (
	"context"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func TestDocumentRuntimeInstallerRequiresIndependentPinAndTrustedAttach(t *testing.T) {
	selected:=&Backend{}
	if _,err:=NewDocumentRuntimeInstaller(selected,"",DocumentAttachConfiguration{Headers:map[string]string{}});err==nil{t.Fatal("missing independently supplied manifest pin accepted")}
	if _,err:=NewDocumentRuntimeInstaller(selected,strings.Repeat("a",64),DocumentAttachConfiguration{});err==nil{t.Fatal("unspecified attach profile accepted")}
	if _,err:=NewDocumentRuntimeInstaller(selected,strings.Repeat("a",64),DocumentAttachConfiguration{Headers:map[string]string{"Host":"different-engine"}});err==nil{t.Fatal("endpoint-changing attach header accepted")}
	installer,err:=NewDocumentRuntimeInstaller(selected,strings.Repeat("a",64),DocumentAttachConfiguration{Headers:map[string]string{}})
	if err!=nil{t.Fatal(err)}
	if result,err:=installer.BuildRuntimeImage(context.Background(),backend.PreparedRuntimeImage{});err==nil||!result.Settled{t.Fatal("unsupported advanced build was offered")}
	if _,err:=installer.DocumentImageID(context.Background(),backend.PreparedRuntimeImage{},"sha256:"+strings.Repeat("b",64));err==nil{t.Fatal("missing settled package and publication accepted")}
}
