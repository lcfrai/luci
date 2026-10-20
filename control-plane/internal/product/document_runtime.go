package product

import (
 "errors"
 "os"
 "path/filepath"
 "github.com/lcfrai/luci/control-plane/internal/hostpath"
 "github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// The request selects only a closed operation kind. Source and expected trust
// pin always come from the owning installation, never a browser path or hash.
func(s *Server)documentRuntimeSource()(string,error){
 if !runtimeinstall.InputProofHex(s.documentRuntime.ImageManifestSHA256)||!filepath.IsAbs(s.runtimeDir){return "",errors.New("the independently verified document runtime package is not configured in this installation")}
 root,err:=hostpath.ResolveOutside(s.projectRoot(),s.runtimeDir);if err!=nil{return "",err}
 root=filepath.Join(root,"document-image")
 for _,name:=range []string{"document-runtime-manifest-v1.json","document-image-linux-amd64.tar"}{
  file:=filepath.Join(root,name);resolved,e:=hostpath.Confine(root,file);if e!=nil||resolved!=file{return "",errors.New("document runtime package path changed")}
  info,e:=os.Lstat(file);if e!=nil||!info.Mode().IsRegular()||info.Size()<1{return "",errors.New("the verified document runtime companion is missing; reinstall a complete package")}
 }
 return root,nil
}

func(s *Server)DocumentRuntimeSource()(string,error){return s.documentRuntimeSource()}
