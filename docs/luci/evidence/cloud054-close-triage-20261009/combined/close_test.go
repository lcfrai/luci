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
 sandboxfixture "github.com/lcfrai/luci/sandboxd"
)
func TestC054RealPTYNormalCloseAcrossOwnerGateway(t *testing.T) {
 for _,gateway:=range []bool{false,true}{ name:="direct-api";if gateway{name="owner-gateway"};t.Run(name,func(t *testing.T){
  f:=attachNewFixture(t,nil,nil);ctx,cancel:=context.WithTimeout(context.Background(),8*time.Second);defer cancel()
  actual,closeActual,err:=sandboxfixture.ControlledServer("/bin/bash",nil,attachTestToken);if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = closeActual()})
  upstream:=httptest.NewServer(actual);t.Cleanup(upstream.Close);f.be.sb.Endpoint=strings.TrimPrefix(upstream.URL,"http://")
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
  if err:=c.Write(ctx,websocket.MessageBinary,append([]byte{1},[]byte("printf 'FINAL-C054\\n'; exit 0\n")...));err!=nil{t.Fatal(err)}
  var output strings.Builder
  for { _,got,err:=c.Read(ctx);if err!=nil{t.Fatalf("before typed Exit: %v",err)};if len(got)==0{t.Fatal("empty frame")};if got[0]==3{if string(got[1:])!="exit status 0"{t.Fatalf("exit %q",got[1:])};break};if got[0]!=0{t.Fatal("unexpected frame")};output.Write(got[1:]) }
  if !strings.Contains(output.String(),"FINAL-C054"){t.Fatal("missing final stdout")}
  _,got,err:=c.Read(ctx);if len(got)!=0||websocket.CloseStatus(err)!=websocket.StatusNormalClosure{t.Fatalf("close after real PTY Exit: code%d %v",websocket.CloseStatus(err),err)}
  if f.be.destroyCount()!=0{t.Fatal("detach destroyed allocation")}
 }) }
}
var _ http.Handler
