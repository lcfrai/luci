package product

import (
 "bytes"
 "encoding/json"
 "io"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

func decodeRuntimeJobState(data []byte,j *RuntimeJob)error{
 if len(data)>32<<10{return runtimeinstall.ErrInputProofOwnership}
 d:=json.NewDecoder(bytes.NewReader(data));tokens:=0
 var value func(int)error
 value=func(depth int)error{tokens++;if depth>16||tokens>8192{return runtimeinstall.ErrInputProofOwnership};t,e:=d.Token();if e!=nil{return e};switch t{case json.Delim('{'):
  seen:=map[string]bool{};for d.More(){k,e:=d.Token();if e!=nil{return e};s,ok:=k.(string);if !ok||seen[s]{return runtimeinstall.ErrInputProofOwnership};seen[s]=true;if e=value(depth+1);e!=nil{return e}};_,e=d.Token();return e
 case json.Delim('['):for d.More(){if e=value(depth+1);e!=nil{return e}};_,e=d.Token();return e};return nil}
 if value(0)!=nil{return runtimeinstall.ErrInputProofOwnership};if _,e:=d.Token();e!=io.EOF{return runtimeinstall.ErrInputProofOwnership}
 var decoded RuntimeJob
 if e:=json.Unmarshal(data,&decoded);e!=nil{return e}
 if decoded.OperationKind==runtimeinstall.DebianInputProofKind||decoded.InputProof!=nil{d=json.NewDecoder(bytes.NewReader(data));d.DisallowUnknownFields();if e:=d.Decode(&decoded);e!=nil{return e}}
 if e:=validateRuntimeJobKind(decoded);e!=nil{return e};*j=decoded;return nil
}
