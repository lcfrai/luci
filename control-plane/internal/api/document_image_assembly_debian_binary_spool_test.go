package api

import (
 "strings"
 "testing"
)

// Pure quota/metadata only. These tests cannot mint a native namespace or run.
func TestDebianBinarySpoolRequestedWriteDebitNeverRefunds(t *testing.T){
 s:=&debianBinarySpoolSet{busy:true}
 if s.debitWrite(10)!=nil||s.writeDebit!=10{t.Fatal("initial request not fully charged")}
 // A native short acknowledgement3 affects its offset only; the next7-byte
 // request is a NEW charge. There is deliberately no refund method.
 if s.debitWrite(7)!=nil||s.writeDebit!=17||s.writeCalls!=2{t.Fatal("short success refunded requested bytes")}
 s.writeDebit=debianBinarySpoolWriteBytes-1
 if s.debitWrite(2)==nil||s.writeDebit!=debianBinarySpoolWriteBytes-1{t.Fatal("byte ceiling mutated on refusal")}
 if s.debitWrite(1)!=nil||s.debitWrite(1)==nil{t.Fatal("exact byte boundary")}
 s.writeDebit=0;s.writeCalls=debianBinarySpoolWriteCalls
 if s.debitWrite(1)==nil||s.writeCalls!=debianBinarySpoolWriteCalls{t.Fatal("call cap reset")}
}
func TestDebianBinarySpoolWriteRefusalsKeepCounters(t *testing.T){
 for _,kind:=range []string{"zero-request","oversize-request","not-borrowed","retired","sticky-failure"}{t.Run(kind,func(t *testing.T){
  s:=&debianBinarySpoolSet{busy:true,writeDebit:10,writeCalls:1};r:=1
  switch kind{case "zero-request":r=0;case "oversize-request":r=65537;case "not-borrowed":s.busy=false;case "retired":s.retiring=true;case "sticky-failure":s.failed=true}
  if s.debitWrite(r)==nil||s.writeDebit!=10||s.writeCalls!=1{t.Fatal("refusal reset quota")}
 })}
}
func TestDebianBinarySpoolFixedObjectsAndByteCeilings(t *testing.T){
 if debianBinarySpoolObjects!=25||debianBinarySpoolTickets!=75||debianBinarySpoolBytes!=16<<20+14*(64<<10){t.Fatal("raw envelope arithmetic drift")}
 if len(debianBinaryRawMagic)!=11||11+23*38!=885{t.Fatal("wire framing drift")}
 seen:=map[string]bool{};for i:=0;i<25;i++{name:=debianBinarySpoolName(i);if name==""||seen[name]||strings.ContainsAny(name,"/\\"){t.Fatal("nonfixed output namespace")};seen[name]=true}
 if debianBinarySpoolName(-1)!=""||debianBinarySpoolName(25)!=""{t.Fatal("arbitrary output ordinal")}
 s:=&debianBinarySpoolSet{}
 if s.storeObject(23,0,binarySpoolSHA(nil))!=nil||s.objects[23].SHA256!="e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"{t.Fatal("actual empty metadata")}
 if s.storeObject(23,0,binarySpoolSHA(nil))==nil||s.storeObject(1,0,binarySpoolSHA(nil))==nil||s.storeObject(1,65537,strings.Repeat("a",64))==nil{t.Fatal("duplicate/empty/oversize accepted")}
}
