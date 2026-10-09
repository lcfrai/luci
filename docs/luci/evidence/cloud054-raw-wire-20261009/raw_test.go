package api
import (
 "bytes"
 "bufio"
 "crypto/rand"
 "encoding/binary"
 "fmt"
 "io"
 "net"
 "net/url"
 "unicode/utf8"
 "context"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
 "github.com/lcfrai/luci/control-plane/internal/management"
 sandboxfixture "github.com/lcfrai/luci/sandboxd"
)
func TestC054RawRFC6455CloseAcrossOwnerGateway(t *testing.T) {
 for _,gateway:=range []bool{false,true}{ name:="direct-api";if gateway{name="owner-gateway"};t.Run(name,func(t *testing.T){
  f:=attachNewFixture(t,nil,nil);ctx,cancel:=context.WithTimeout(context.Background(),8*time.Second);defer cancel()
  actual,closeActual,err:=sandboxfixture.ControlledServer("/bin/bash",nil,attachTestToken);if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = closeActual()})
  upstream:=httptest.NewServer(actual);t.Cleanup(upstream.Close);f.be.sb.Endpoint=strings.TrimPrefix(upstream.URL,"http://")
  var c *c054RawClient
  if !gateway { c=c054RawDial(t,ctx,f.srv.URL+"/v1/sessions/"+f.id+"/attach") } else {
   dir:=t.TempDir(); g,err:=management.New(management.Options{StateDir:dir,InitialHandler:f.api.guarded,Version:"controlled-c054"});if err!=nil{t.Fatal(err)};t.Cleanup(func(){shutdown,stop:=context.WithTimeout(context.Background(),time.Second);defer stop();if err:=g.Close(shutdown);err!=nil{t.Error(err)}})
   h:=g.Handler(); request:=func(path,token string,body any,status int) []byte {t.Helper();b,_:=json.Marshal(body);r:=httptest.NewRequest("POST",path,bytes.NewReader(b));r.Header.Set("Content-Type","application/json");if token!=""{r.Header.Set("Authorization","Bearer "+token)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);if w.Code!=status{t.Fatalf("%s status%d want%d",path,w.Code,status)};return w.Body.Bytes()}
   secret,err:=os.ReadFile(filepath.Join(dir,"control-plane","bootstrap-token"));if err!=nil{t.Fatal(err)}
   var auth struct{Token string `json:"token"`};err=json.Unmarshal(request("/v1/auth/bootstrap","",map[string]string{"bootstrapToken":strings.TrimSpace(string(secret)),"username":"owner","displayName":"Fixture owner","password":"c054 isolated fixture password"},201),&auth);if err!=nil||auth.Token==""{t.Fatal("bootstrap fixture")}
   path:="/v1/sessions/"+f.id+"/attach";var ticket struct{Ticket string `json:"ticket"`};if err:=json.Unmarshal(request("/v1/auth/socket-ticket",auth.Token,map[string]string{"path":path},200),&ticket);err!=nil||ticket.Ticket==""{t.Fatal("socket ticket")}
   srv:=httptest.NewServer(h);t.Cleanup(srv.Close)
   c=c054RawDial(t,ctx,srv.URL+path+"?ticket="+ticket.Ticket)
  }
  // Exact three frozen scenario commands with an inert24-byte nonce; send205
  // separately masked one-byte stdin frames and retain the ready/body barriers.
  commands := []string{"stty -echo; printf 'C054_READY_%s\\n' '000000000000000000000000'\r","printf 'C054_BODY_EXIT_%s {\"type\":\"exit\",\"code\":0}\\n' '000000000000000000000000'\r","printf 'C054_FIRST_%s\\n' '000000000000000000000000'; exit 0\r"}
  total := 0
  for index,command := range commands {
   for i:=range command { c.send(t,2,[]byte{1,command[i]}); total++ }
   if index<2 { marker:=[]string{"C054_READY_","C054_BODY_EXIT_"}[index]+strings.Repeat("0",24);var seen strings.Builder
    for !strings.Contains(seen.String(),marker){op,got:=c.read(t);if op!=2||len(got)<1||got[0]!=0{t.Fatal("wrong frame before command barrier")};seen.Write(got[1:])}
   }
  }
  if total!=205{t.Fatalf("input count=%d",total)}
  var output strings.Builder
  for { op,got:=c.read(t);if op!=2||len(got)==0{t.Fatalf("before typed Exit op=%d length=%d",op,len(got))};if got[0]==3{if string(got[1:])!="exit status 0"{t.Fatalf("exit %q",got[1:])};break};if got[0]!=0{t.Fatal("unexpected frame")};output.Write(got[1:]) }
  if !strings.Contains(output.String(),"C054_FIRST_"+strings.Repeat("0",24)){t.Fatal("missing final stdout")}
  op,got:=c.read(t)
  if op!=8||!bytes.Equal(got,[]byte{3,232})||!utf8.Valid(got[2:]){t.Fatalf("after Exit: opcode=%d bytes=%x, want exact normal close",op,got)}
  t.Logf("server close is exactly 88 02 03 e8; one-byte stdin frames=%d",total)
  c.send(t,8,got) // Normal peer acknowledgement only after exact close validation.
  if f.be.destroyCount()!=0{t.Fatal("detach destroyed allocation")}
 }) }
}
var _ http.Handler

// Independently decode the uncompressed short frames used by this controlled fixture.
// The decoder rejects masked server data, reserved bits and fragmented control/data.
type c054RawClient struct { conn net.Conn; reader *bufio.Reader }
func c054RawDial(t *testing.T, ctx context.Context, address string) *c054RawClient {
 t.Helper();u,err:=url.Parse(address);if err!=nil{t.Fatal(err)}
 conn,err:=(&net.Dialer{}).DialContext(ctx,"tcp",u.Host);if err!=nil{t.Fatal(err)};t.Cleanup(func(){_ = conn.Close()})
 deadline,_:=ctx.Deadline();if err:=conn.SetDeadline(deadline);err!=nil{t.Fatal(err)}
 // Browser-like compression offer must remain unaccepted in this production path.
 _,err=fmt.Fprintf(conn,"GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nSec-WebSocket-Extensions: permessage-deflate; client_max_window_bits\r\n\r\n",u.RequestURI(),u.Host);if err!=nil{t.Fatal(err)}
 reader:=bufio.NewReader(conn);req,_:=http.NewRequest("GET",address,nil);response,err:=http.ReadResponse(reader,req);if err!=nil{t.Fatal(err)}
 if response.StatusCode!=101||response.Header.Get("Sec-WebSocket-Accept")!="s3pPLMBiTxaQ9kYGzzhZRbK+xOo="||response.Header.Get("Sec-WebSocket-Extensions")!=""{t.Fatal("unexpected handshake")}
 return &c054RawClient{conn:conn,reader:reader}
}
func(c *c054RawClient)send(t *testing.T,opcode byte,payload []byte){
 t.Helper();if len(payload)>125{t.Fatal("fixture sends only short frames")};var mask [4]byte;if _,err:=rand.Read(mask[:]);err!=nil{t.Fatal(err)}
 b:=[]byte{0x80|opcode,0x80|byte(len(payload))};b=append(b,mask[:]...);for i,v:=range payload{b=append(b,v^mask[i%4])};if _,err:=c.conn.Write(b);err!=nil{t.Fatal(err)}
}
func(c *c054RawClient)read(t *testing.T)(byte,[]byte){
 t.Helper();var h [2]byte;if _,err:=io.ReadFull(c.reader,h[:]);err!=nil{t.Fatal(err)}
 if h[0]&0xf0!=0x80||h[1]&0x80!=0{t.Fatalf("invalid frame flags %02x %02x",h[0],h[1])};opcode:=h[0]&15;size:=uint64(h[1]&127)
 if size==126{var x [2]byte;if _,err:=io.ReadFull(c.reader,x[:]);err!=nil{t.Fatal(err)};size=uint64(binary.BigEndian.Uint16(x[:]))}else if size==127{var x [8]byte;if _,err:=io.ReadFull(c.reader,x[:]);err!=nil{t.Fatal(err)};size=binary.BigEndian.Uint64(x[:])}
 if size>1<<20||opcode>=8&&size>125{t.Fatal("oversize frame")};p:=make([]byte,int(size));if _,err:=io.ReadFull(c.reader,p);err!=nil{t.Fatal(err)};return opcode,p
}
