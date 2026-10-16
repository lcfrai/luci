//go:build !windows

package api

import (
 "context"
 "testing"
)

func TestDebianCensusUnsupportedPlatformCannotIssueOrStartObserver(t *testing.T){
 n:=newDebianProofNativeOwner(context.Background());if _,err:=n.reserveNativeOriginal(debianProofOriginalFile);err==nil{t.Fatal("unsupported original issuer")};if n.startObserver(context.Background(),nil)==nil||n.initialAdmission(context.Background())==nil||n.originalCensusSettled(){t.Fatal("unsupported platform fabricated native proof")}
}
