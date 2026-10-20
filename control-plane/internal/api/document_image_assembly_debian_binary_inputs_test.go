package api

import (
 "context"
 "encoding/json"
 "fmt"
 "path/filepath"
 "sort"
 "testing"
)

// These are declared identities, not fake SHA preimages or actual packages.
// The test exercises only the pre-Acquire mapping/permission boundary.
func binaryDeclaredPathsFixture(t *testing.T)(*debianBinaryProducerInputs,debianBinaryInputPaths){
 t.Helper();selection,_,_,_,_:=assemblyBinaryCoverageFixture(t)
 _,indexes,policy,_,_:=assemblyIndexMetadataFixture(t)
 b:=&debianBinaryProducerInputs{selection:selection,indexSelection:indexes,policy:policy}
 paths:=debianBinaryInputPaths{Schema:1}
 for _,p:=range selection.Packages{paths.Files=append(paths.Files,debianBinaryInputPath{fmt.Sprintf("package.%03d",p.Ordinal),fmt.Sprintf("packages/%03d.deb",p.Ordinal),p.Bytes,p.SHA256})}
 for _,v:=range indexes.Objects{paths.Files=append(paths.Files,debianBinaryInputPath{"xz-index:"+v.SHA256,"indexes/"+v.SHA256+".xz",v.Bytes,v.SHA256})}
 sort.Slice(paths.Files,func(i,j int)bool{return paths.Files[i].Role<paths.Files[j].Role})
 return b,paths
}
func TestBinaryDeclaredPathsRequireExactSelectionBeforeAcquisition(t *testing.T){
 b,paths:=binaryDeclaredPathsFixture(t);root:=filepath.Join(t.TempDir(),"unopened-input-root");raw,_:=json.Marshal(paths)
 packages,indexes,e:=debianBinaryDeclaredObjects(raw,root,b)
 if e!=nil||len(packages)!=174||len(indexes)!=8||packages[0].Role!="package.000"||packages[173].Role!="package.173"{t.Fatal(e,len(packages),len(indexes))}
 for _,kind:=range []string{"escape","absolute","backslash","alternate-stream","windows-device","trailing-dot","same-path","wrong-digest","extra-role","reordered","noncanonical","unknown-field"}{t.Run(kind,func(t *testing.T){
  changed:=paths;changed.Files=append([]debianBinaryInputPath(nil),paths.Files...)
  switch kind{case "escape":changed.Files[0].RelativePath="../outside";case "absolute":changed.Files[0].RelativePath="/outside";case "backslash":changed.Files[0].RelativePath=`packages\outside`;case "alternate-stream":changed.Files[0].RelativePath="packages/a:stream";case "windows-device":changed.Files[0].RelativePath="packages/CON.deb";case "trailing-dot":changed.Files[0].RelativePath="packages/input.deb.";case "same-path":changed.Files[1].RelativePath=changed.Files[0].RelativePath;case "wrong-digest":changed.Files[0].SHA256=changed.Files[1].SHA256;case "extra-role":changed.Files[0].Role="unselected-package";case "reordered":changed.Files[0],changed.Files[1]=changed.Files[1],changed.Files[0]}
  data,_:=json.Marshal(changed);if kind=="noncanonical"{data=append(data,'\n')};if kind=="unknown-field"{data=append(data[:len(data)-1],[]byte(`,"acquired":true}`)...)}
  if _,_,e:=debianBinaryDeclaredObjects(data,root,b);e==nil{t.Fatal("unselected or escaping input gained original acquisition")}
 })}
}

func TestBinaryClosurePermitRequiresExactlyOneLiveTypedArm(t *testing.T){
 for _,p:=range []*assemblyDebianPublicationPermit{nil,{}, {candidate:&assemblyDebianDomainCandidate{},accepted:&assemblyAcceptedInputCandidate{}},{indexes:&assemblyDebianIndexCandidate{},accepted:&assemblyAcceptedInputCandidate{}}}{if p.oneArm(){t.Fatal("ambiguous permit accepted")}}
 p:=&assemblyDebianPublicationPermit{accepted:&assemblyAcceptedInputCandidate{}}
 if !p.oneArm()||p.begin(context.Background(),nil,assemblyCatalogCAS{})==nil{t.Fatal("membership/empty candidate became adoption authority")}
}
