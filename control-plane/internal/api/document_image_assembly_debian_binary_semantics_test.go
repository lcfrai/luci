package api

import (
 "context"
 "testing"
)

func TestBinarySemanticCaptureCannotUpgradeRawFramesOrCopiedExit(t *testing.T){
 // These are deliberately incomplete lower-layer values. Neither a copied
 // zero exit nor raw framing can authorize reading evidence or publishing.
 for _,boundary:=range []string{"missing-api","missing-plan","missing-spool","missing-original-owner","copied-exit"}{t.Run(boundary,func(t *testing.T){
  a,p,s:=&API{},&debianProducerPlan{},&debianBinarySpoolSet{}
  child:=debianProofChildResult{}
  switch boundary{case "missing-api":a=nil;case "missing-plan":p=nil;case "missing-spool":s=nil;case "copied-exit":s.owner=newDebianBinaryProofNativeOwner(context.Background());child.ExitConfirmed=true;child.JobEmpty=true;child.HandlesClosed=true;child.CaptureSettled=true;child.OriginalHandleOwned=true}
  if got,err:=captureDebianBinarySemantics(context.Background(),a,p,s,child,assemblyDebianSignedIndexBindingsV1{});err==nil||got!=nil{t.Fatal("metadata fabricated original semantic capture")}
 })}
}

func TestBinarySemanticReadRefusesUnfinishedRetiredBusyOrArbitrarySlot(t *testing.T){
 for _,slot:=range []int{-1,0,22,24,25}{
  s:=&debianBinarySpoolSet{finished:true}
  if got,err:=s.readSemanticObject(context.Background(),slot);err==nil||got!=nil{t.Fatal("missing original guard returned evidence")}
  if s.busy{t.Fatal("refused metadata retained a synthetic borrower")}
 }
}
