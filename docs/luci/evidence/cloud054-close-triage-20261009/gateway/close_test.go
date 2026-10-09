package api
import (
 "bytes"
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
 "github.com/coder/websocket"
 "github.com/lcfrai/luci/control-plane/internal/management"
)
func TestC054NormalCloseThroughAttachAndOwnerGateway(t *testing.T) {
 for _,gateway:=range []bool{false,true}{ name:="direct-api";if gateway{name="owner-gateway"};t.Run(name,func(t *testing.T){
  f:=attachNewFixture(t,nil,nil);ctx,cancel:=context.WithTimeout(context.Background(),8*time.Second);defer cancel()
  var c *websocket.Conn
  if !gateway { c=f.dial(ctx,t) } else {
   dir:=t.TempDir(); g,err:=management.New(management.Options{StateDir:dir,InitialHandler:f.api.guarded,Version:"controlled-c054"});if err!=nil{t.Fatal(err)};t.Cleanup(func(){shutdown,stop:=context.WithTimeout(context.Background(),time.Second);defer stop();if err:=g.Close(shutdown);err!=nil{t.Error(err)}})
   h:=g.Handler(); request:=func(path,token string,body any,status int) []byte {t.Helper();b,_:=json.Marshal(body);r:=httptest.NewRequest("POST",path,bytes.NewReader(b));r.Header.Set("Content-Type","application/json");if token!=""{r.Header.Set("Authorization","Bearer "+token)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=status{t.Fatalf("%s status%d want%d",path,w.Code,status)};return w.Body.Bytes()}
   secret,err:=os.ReadFile(filepath.Join(dir,"control-plane","bootstrap-token"));if err!=nil{t.Fatal(err)}
   var auth struct{Token string `json:"token"`};err=json.Unmarshal(request("/v1/auth/bootstrap","",map[string]string{"bootstrapToken":strings.TrimSpace(string(secret)),"username":"owner","displayName":"Fixture owner","password":"c054 isolated fixture password"},201),&auth);if err!=nil||auth.Token==""{t.Fatal("bootstrap fixture")}
   path:="/v1/sessions/"+f.id+"/attach";var ticket struct{Ticket string `json:"ticket"`};if err:=json.Unmarshal(request("/v1/auth/socket-ticket",auth.Token,map[string]string{"path":path},200),&ticket);err!=nil||ticket.Ticket==""{t.Fatal("socket ticket")}
   srv:=httptest.NewServer(h);t.Cleanup(srv.Close)
   c,_,err=websocket.Dial(ctx,"ws"+strings.TrimPrefix(srv.URL,"http")+path+"?ticket="+ticket.Ticket,nil);if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = c.CloseNow()})
  }
  for _,frame:=range [][]byte{[]byte("\x00FINAL-C054"),[]byte("\x03exit status 0")} { f.sandboxd.push(frame);_,got,err:=c.Read(ctx);if err!=nil||!bytes.Equal(got,frame){t.Fatalf("final frame mismatch: %v",err)} }
  f.sandboxd.close<-websocket.StatusNormalClosure
  _,got,err:=c.Read(ctx);if len(got)!=0||websocket.CloseStatus(err)!=websocket.StatusNormalClosure{t.Fatalf("close after Exit: code%d %v",websocket.CloseStatus(err),err)}
  if f.be.destroyCount()!=0{t.Fatal("detach destroyed allocation")}
 }) }
}
var _ http.Handler
