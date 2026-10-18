//go:build !windows

package api

import (
 "context"
 "testing"
)

func TestDebianBinarySpoolUnsupportedPlatformRefusesWithoutOwner(t *testing.T){
 s,err:=newDebianBinarySpoolSet(context.Background(),nil,"",debianBinaryRawDecoder{})
 if err==nil||s!=nil{t.Fatal("unsupported native owner fabricated")}
 s=&debianBinarySpoolSet{}
 if s.persistFrame(context.Background(),debianBinaryRawView{})==nil||s.readStderrOnce(context.Background())==nil||s.finishRawSpool(context.Background())==nil||s.closeRetiredRaw(context.Background())==nil{t.Fatal("unsupported positive output/closure")}
}
