//go:build linux || darwin
package server_test
import (
 "context"
 "fmt"
 "strings"
 "testing"
 "time"
 "github.com/coder/websocket"
 "github.com/lcfrai/luci/sandboxd/internal/server"
 "github.com/lcfrai/luci/sandboxd/internal/wire"
)
func TestC054NaturalPTYExitCompletesNormalWebSocketClose(t *testing.T) {
 for _, code := range []int{0,7} { t.Run(fmt.Sprint(code),func(t *testing.T){
  srv:=newServer(t,server.Config{Command:"/bin/sh",Args:[]string{"-c",fmt.Sprintf("stty -echo; printf READY; read line; printf 'FINAL-C054\\n'; exit %d",code)},AttachToken:testToken})
  c,ctx:=dial(t,srv,testToken)
  readUntil(t,c,ctx,[]byte("READY"),5*time.Second)
  if err:=c.Write(ctx,websocket.MessageBinary,wire.Encode(wire.Stdin,[]byte("\n")));err!=nil{t.Fatal(err)}
  var output strings.Builder
  bounded,cancel:=context.WithTimeout(ctx,5*time.Second);defer cancel()
  for { _,frame,err:=c.Read(bounded);if err!=nil{t.Fatalf("before typed Exit: %v",err)};kind,payload,err:=wire.Decode(frame);if err!=nil{t.Fatal(err)};if kind==wire.Exit { if string(payload)!=fmt.Sprintf("exit status %d",code){t.Fatalf("wrong exit: %q",payload)};break }; if kind!=wire.Stdout{t.Fatal("unexpected frame")};output.Write(payload) }
  if !strings.Contains(output.String(),"FINAL-C054"){t.Fatal("missing final stdout")}
  _,frame,err:=c.Read(bounded)
  if len(frame)!=0||websocket.CloseStatus(err)!=websocket.StatusNormalClosure{t.Fatalf("natural PTY exit lost normal close: status=%d data=%q err=%v",websocket.CloseStatus(err),frame,err)}
 }) }
}
