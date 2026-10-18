package api

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "path/filepath"
 "sync"
 "time"

 "github.com/lcfrai/luci/control-plane/internal/backend"
)

const debianBinarySpoolObjects = 25
const debianBinarySpoolTickets = 75
const debianBinarySpoolBytes = 17694720
const debianBinarySpoolWriteBytes = 20 << 20
const debianBinarySpoolWriteCalls = 32768
var errDebianBinarySpool = errors.New("binary raw spool incomplete or originals unsettled")

// One fixed pool in the SAME census. claimed is operation ownership; it is
// never a native success flag. All partial/unknown outcomes use real tickets.
type debianBinarySpoolPool struct {
 owner *debianProofNativeOwner
 tickets [25][3]*debianProofOriginalTicket
 claimed [25][3]bool
 sealed bool
}
func(o *debianProofNativeOwner)reserveBinarySpoolPool()(*debianBinarySpoolPool,error){
 if o==nil||o.census==nil{return nil,errDebianProofCensus};c:=o.census;c.mu.Lock();defer c.mu.Unlock()
 if c.owner!=o||c.spoolPool!=nil||c.retiring||o.nativeRetired()||!time.Now().Before(o.workLimit())||c.reserved>1024-75||len(c.native)>1024-75{return nil,errDebianProofCensus}
 p:=&debianBinarySpoolPool{owner:o}
 for i:=0;i<25;i++{for k:=0;k<3;k++{kind:=debianProofOriginalFile;if k==1{kind=debianProofOriginalReOpen};t:=&debianProofOriginalTicket{owner:o,kind:kind,pending:true,slots:make([]debianProofOriginalSlot,1)};p.tickets[i][k]=t;c.native=append(c.native,t)}}
 c.spoolPool=p;c.reserved+=75;return p,nil
}
func(p *debianBinarySpoolPool)claim(object,kind int)(*debianProofOriginalTicket,error){
 if p==nil||p.owner==nil||p.owner.census==nil||object<0||object>=25||kind<0||kind>=3{return nil,errDebianProofCensus};c:=p.owner.census;c.mu.Lock();defer c.mu.Unlock()
 t:=p.tickets[object][kind];if c.spoolPool!=p||p.sealed||p.claimed[object][kind]||t==nil||!c.hasNativeLocked(t)||!t.pending||c.retiring||p.owner.nativeRetired()||!time.Now().Before(p.owner.workLimit()){return nil,errDebianProofCensus}
 p.claimed[object][kind]=true;t.attempted=true;return t,nil
}
func(p *debianBinarySpoolPool)seal(){
 if p==nil||p.owner==nil||p.owner.census==nil{return};c:=p.owner.census;c.mu.Lock();defer c.mu.Unlock();if c.spoolPool!=p||p.sealed{return};p.sealed=true
 for i:=0;i<25;i++{for k:=0;k<3;k++{t:=p.tickets[i][k];if !p.claimed[i][k]&&t!=nil&&c.hasNativeLocked(t)&&t.pending{t.pending=false;t.slots[0]=debianProofOriginalSlot{};c.reserved--}}}
}
func(p *debianBinarySpoolPool)settled()bool{
 if p==nil||p.owner==nil||p.owner.census==nil{return false};c:=p.owner.census;c.mu.Lock();defer c.mu.Unlock();if c.spoolPool!=p||!p.sealed{return false}
 for i:=0;i<25;i++{for k:=0;k<3;k++{t:=p.tickets[i][k];if t==nil||!c.hasNativeLocked(t)||t.pending{return false};for _,slot:=range t.slots{if slot.value!=0||slot.closing||slot.unknown{return false}}}};return true
}

type debianBinarySpoolObject struct {
 Role string `json:"role"`
 Bytes int64 `json:"bytes"`
 SHA256 string `json:"sha256"`
}
type debianBinarySpoolManifest struct {
 Schema string `json:"schema"`
 Meaning string `json:"meaning"`
 Frames [23]debianBinaryRawRef `json:"frames"`
 Stderr debianBinarySpoolObject `json:"stderr"`
 ReadAttempts uint32 `json:"readAttempts"`
 StdoutDebit int `json:"stdoutDebit"`
 StderrDebit int `json:"stderrDebit"`
}
// Private mechanical raw retention only. No constructor mints a binary
// purpose, original completion, trusted input, domain candidate or live caller.
type debianBinarySpoolSet struct {
 mu sync.Mutex
 owner *debianProofNativeOwner
 decoder debianBinaryRawDecoder
 namespace *debianProofNamespaceAdmission
 namespaceGuard backend.DocumentImageAssemblyNamespaceGuard
 output *debianProofEvidenceAdmission
 outputGuard backend.DocumentImageAssemblyEvidenceGuard
 pool *debianBinarySpoolPool
 native *debianBinarySpoolNative
 objects [25]debianBinarySpoolObject
 nextFrame int
 retainedBytes int64
 writeDebit,writeCalls int
 busy,retiring,failed,stderrEOF,finished,closing bool
 stderrPending bool
 stderrBytes int
 stderr [64<<10]byte
 writeBacking [64<<10]byte
 readBacking [1]byte
}
func(s *debianBinarySpoolSet)current(ctx context.Context)bool{return s!=nil&&s.owner!=nil&&ctx!=nil&&ctx.Err()==nil&&!s.owner.nativeRetired()&&time.Now().Before(s.owner.workLimit())}
func(s *debianBinarySpoolSet)begin(ctx context.Context)error{
 if s==nil{return errDebianBinarySpool};s.mu.Lock();defer s.mu.Unlock()
 if !s.current(ctx)||s.busy||s.retiring||s.failed||s.finished||s.closing||s.pool==nil||s.namespaceGuard==nil{return errDebianBinarySpool};s.busy=true;return nil
}
func(s *debianBinarySpoolSet)end(err error)error{s.mu.Lock();late:=s.retiring||s.owner.nativeRetired()||!time.Now().Before(s.owner.workLimit());if err!=nil||late{s.failed=true;s.finished=false};s.busy=false;s.mu.Unlock();if err==nil&&late{err=errDebianBinarySpool};if err!=nil{s.owner.retire()};return err}
func(s *debianBinarySpoolSet)debitWrite(requested int)error{
 s.mu.Lock();defer s.mu.Unlock()
 if !s.busy||s.retiring||s.failed||requested<1||requested>len(s.writeBacking)||s.writeCalls>=debianBinarySpoolWriteCalls||requested>debianBinarySpoolWriteBytes-s.writeDebit{return errDebianBinarySpool}
 s.writeCalls++;s.writeDebit+=requested;return nil // NEVER refund a valid short write.
}
func debianBinarySpoolName(slot int)string{if slot>=0&&slot<23{return fmt.Sprintf("frame-%02d.raw",slot)};if slot==23{return "stderr.raw"};if slot==24{return "wire-manifest.json"};return ""}
func debianBinarySpoolRole(slot int)string{if slot>=0&&slot<23{return fmt.Sprintf("binary-raw-frame.%02d",slot)};if slot==23{return "binary-raw-stderr"};if slot==24{return "binary-raw-manifest"};return ""}
func(s *debianBinarySpoolSet)path(slot int)string{if s.namespace==nil||s.namespace.source==nil||debianBinarySpoolName(slot)==""{return ""};return filepath.Join(s.namespace.source.namespacePath,debianBinarySpoolName(slot))}
func(s *debianBinarySpoolSet)storeObject(slot int,bytes int,digest string)error{
 s.mu.Lock();defer s.mu.Unlock();if slot<0||slot>=25||bytes<0||s.objects[slot].Role!=""||!assemblyCatalogHex(digest)||int64(bytes)>debianBinarySpoolBytes-s.retainedBytes{return errDebianBinarySpool}
 maximum:=64<<10;if slot<23&&slot%2==0&&slot<22{maximum=4<<20};if bytes>maximum||bytes==0&&slot!=23{return errDebianBinarySpool}
 s.objects[slot]=debianBinarySpoolObject{debianBinarySpoolRole(slot),int64(bytes),digest};s.retainedBytes+=int64(bytes);return nil
}
func(s *debianBinarySpoolSet)evidenceObjects()([]backend.DocumentImageAssemblyEvidenceObject,error){
 s.mu.Lock();defer s.mu.Unlock();out:=make([]backend.DocumentImageAssemblyEvidenceObject,25)
 for i,v:=range s.objects{if v.Role!=debianBinarySpoolRole(i)||!assemblyCatalogHex(v.SHA256){return nil,errDebianBinarySpool};out[i]=backend.DocumentImageAssemblyEvidenceObject{Role:v.Role,Path:s.path(i),Bytes:v.Bytes,Digest:v.SHA256}}
 return out,nil
}
func(s *debianBinarySpoolSet)manifestBytes()([]byte,error){
 o:=s.decoder.observation();s.mu.Lock();defer s.mu.Unlock()
 if o.State!="raw-framing-complete-not-native-or-semantic-proof"||o.Frames!=23||o.Pending||s.nextFrame!=23||!s.stderrEOF||s.objects[23].Role==""{return nil,errDebianBinarySpool}
 m:=debianBinarySpoolManifest{"luci-binary-raw-spool-v1","raw-only-not-native-semantic-or-adoption-proof",o.Refs,s.objects[23],o.ReadAttempts,o.StdoutDebit,o.StderrDebit}
 b,e:=json.Marshal(m);if e!=nil||len(b)>64<<10{return nil,errDebianBinarySpool};return b,nil
}
func(s *debianBinarySpoolSet)requestRetirement(){
 if s==nil{return};s.mu.Lock();s.retiring=true;p:=s.pool;ns:=s.namespace;out:=s.output;s.mu.Unlock()
 // Latches only, never Close/Transfer or a wait for a native borrower.
 if ns!=nil&&ns.source!=nil&&ns.source.namespace!=nil{ns.source.namespace.RequestRetirement()}
 if out!=nil&&out.source!=nil&&out.source.evidence!=nil{out.source.evidence.RequestRetirement()}
 s.decoder.retire() // pure short latch; never waits for native IO/backing
 if p!=nil{p.seal()}
}
func(o *debianProofNativeOwner)retireBinarySpools(){if o==nil{return};o.mu.Lock();sets:=append([]*debianBinarySpoolSet(nil),o.binarySpools...);o.mu.Unlock();for _,s:=range sets{s.requestRetirement()}}
func binarySpoolSHA(b []byte)string{h:=sha256.Sum256(b);return hex.EncodeToString(h[:])}
