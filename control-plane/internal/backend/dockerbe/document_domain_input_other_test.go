//go:build !windows

package dockerbe

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestDocumentDomainInputsUnsupportedPlatformHasNoOwner(t *testing.T){
	ctx,cancel:=context.WithTimeout(context.Background(),time.Second);defer cancel()
	owner,err:=OpenDocumentDomainInputs(ctx,filepath.Join(t.TempDir(),"root"),[]backend.DomainInput{domainTestInput("base","base",0)})
	if err==nil||owner!=nil{t.Fatal("non-Windows path metadata substituted for held originals")}
}
