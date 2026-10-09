// Deterministic, pre-authored acceptance workloads. Not model-authored results.
// No imports, subprocesses, filesystem writes or network work on module load.
const moduleFile = 'module fixture.local/workload\n\ngo 1.26.0\n';
export const redFailureMarker = 'APPROVED_REQUIREMENT_UNIMPLEMENTED';
const fixedFailure = 'APPROVED_REQUIREMENT_UNIMPLEMENTED';

function renderer(amended=false) {
 const suffix=amended?'b.WriteString("<p>Total: "); b.WriteString(strconv.Itoa(len(items))); b.WriteString("</p>")':'';
 return `package workload
import ("html"; "strings"${amended?'; "strconv"':''})
// Render escapes every caller-controlled text node and preserves item order.
func Render(title string, items []string) string {
 var b strings.Builder
 b.WriteString("<section><h1>"); b.WriteString(html.EscapeString(title)); b.WriteString("</h1><ul>")
 for _, item := range items { b.WriteString("<li>"); b.WriteString(html.EscapeString(item)); b.WriteString("</li>") }
 b.WriteString("</ul>"); ${suffix}
 b.WriteString("</section>\\n")
 return b.String()
}
`;
}
function rendererTests(amended=false) {
 return `package workload
import ("strconv"; "strings"; "testing")
// Independent byte-wise oracle: invalid UTF-8 bytes are preserved, as required
// for text escaping; rune iteration would accidentally change that contract.
func oracleEscape(value string) string {
 var out strings.Builder
 for i:=0;i<len(value);i++ { switch value[i] {
 case '&': out.WriteString("&amp;")
 case '<': out.WriteString("&lt;")
 case '>': out.WriteString("&gt;")
 case '\\'': out.WriteString("&#39;")
 case '"': out.WriteString("&#34;")
 default: out.WriteByte(value[i])
 } }
 return out.String()
}
func renderOracle(title string, items []string) string {
 out := "<section><h1>"+oracleEscape(title)+"</h1><ul>"
 for _,item := range items { out += "<li>"+oracleEscape(item)+"</li>" }
 out += "</ul>"
 ${amended?'out += "<p>Total: "+strconv.Itoa(len(items))+"</p>"':'_ = strconv.Itoa // Same imports in amended and initial fixed suites.'}
 return out+"</section>\\n"
}
func TestApprovedRequirement(t *testing.T) {
 cases:=[]struct{title string; items []string}{
  {"A<&",[]string{"first", \u0060quote " and '\u0060}},
  {"",nil},{"unicode π",[]string{"<script>","&already;","last"}},
 }
 for _,c:=range cases { if got,want:=Render(c.title,c.items),renderOracle(c.title,c.items);got!=want { t.Fatalf("${fixedFailure}: render escaped text/order: got %q want %q",got,want) } }
}
func TestDeterministicRendering(t *testing.T) {
 items:=[]string{"one","two"};before:=strings.Join(items,"|");a:=Render("stable",items);b:=Render("stable",items)
 if a!=b||strings.Join(items,"|")!=before {t.Fatal("render mutated its input or changed its result")}
}
func FuzzApprovedInvariant(f *testing.F) {
 f.Add([]byte("title<&|one|two"));f.Add([]byte{0xff,0,39,34,60,38});f.Add([]byte{})
 f.Fuzz(func(t *testing.T,data []byte){
  if len(data)>512 {data=data[:512]};parts:=strings.Split(string(data),"|");if len(parts)>9 {parts=parts[:9]}
  title:=parts[0];if len(title)>128 {title=title[:128]};items:=parts[1:]
  if got,want:=Render(title,items),renderOracle(title,items);got!=want {t.Fatalf("escaping/order oracle mismatch: got %q want %q",got,want)}
 })
}
`;
}
const rendererMain=`package main
import ("fmt"; "fixture.local/workload")
func main(){fmt.Print(workload.Render("Notes & plans",[]string{"<ready>","O'Reilly"}))}
`;

const apiImplementation=`package workload
import ("bytes"; "encoding/json"; "io"; "net/http"; "strings"; "sync")
// NewHandler stores one note per handler and enforces role/method authorization.
func NewHandler() http.Handler {
 var mu sync.Mutex
 value:=""
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/note" {http.NotFound(w,r);return}
  role:=r.Header.Get("X-Role")
  if role!="reader"&&role!="editor" {http.Error(w,"forbidden",http.StatusForbidden);return}
  switch r.Method {
  case http.MethodGet:
   mu.Lock();current:=value;mu.Unlock()
   w.Header().Set("Content-Type","application/json");_ = json.NewEncoder(w).Encode(struct{Value string \u0060json:"value"\u0060}{current})
  case http.MethodPost:
   if role!="editor" {http.Error(w,"forbidden",http.StatusForbidden);return}
   var body struct{Value string \u0060json:"value"\u0060}
   raw,err:=io.ReadAll(io.LimitReader(r.Body,1025));if err!=nil||len(raw)>1024 {http.Error(w,"invalid",http.StatusBadRequest);return}
   decoder:=json.NewDecoder(bytes.NewReader(raw));decoder.DisallowUnknownFields()
   if err:=decoder.Decode(&body);err!=nil||len(body.Value)>256 {http.Error(w,"invalid",http.StatusBadRequest);return}
   var trailing any
   if decoder.Decode(&trailing)!=io.EOF {http.Error(w,"invalid",http.StatusBadRequest);return}
   // The value is data; embedded markup and newlines are preserved, not run.
   mu.Lock();value=strings.Clone(body.Value);mu.Unlock()
   w.Header().Set("Content-Type","application/json");w.WriteHeader(http.StatusCreated)
   _ = json.NewEncoder(w).Encode(body)
  default: w.Header().Set("Allow","GET, POST");http.Error(w,"method",http.StatusMethodNotAllowed)
  }
 })
}
`;
const apiTests=`package workload
import ("bytes"; "encoding/hex"; "encoding/json"; "fmt"; "net/http"; "net/http/httptest"; "testing")
func request(handler http.Handler,role,method,body string)*httptest.ResponseRecorder {
 req:=httptest.NewRequest(method,"/note",bytes.NewBufferString(body));req.Header.Set("X-Role",role)
 recorder:=httptest.NewRecorder();handler.ServeHTTP(recorder,req);return recorder
}
func TestApprovedRequirement(t *testing.T){
 h:=NewHandler()
 cases:=[]struct{role,method,body string;status int; value string}{
  {"guest","GET","",403,""},{"reader","POST",\u0060{"value":"bad"}\u0060,403,""},
  {"editor","POST",\u0060{"value":"approved"}\u0060,201,"approved"},
  {"reader","GET","",200,"approved"},{"editor","POST","broken",400,""},
  {"editor","POST",\u0060{"value":"bad","unknown":true}\u0060,400,""},
  {"reader","GET","",200,"approved"},{"editor","DELETE","",405,""},
 }
 for i,c:=range cases {r:=request(h,c.role,c.method,c.body);if r.Code!=c.status {t.Fatalf("${fixedFailure}: request %d status %d want %d",i,r.Code,c.status)};if c.status<300 {var out struct{Value string \u0060json:"value"\u0060};if json.Unmarshal(r.Body.Bytes(),&out)!=nil||out.Value!=c.value {t.Fatalf("${fixedFailure}: request %d lost saved value",i)}}}
}
func TestHandlerStateIsIsolated(t *testing.T){
 first,second:=NewHandler(),NewHandler();request(first,"editor","POST",\u0060{"value":"private"}\u0060)
 r:=request(second,"reader","GET","");if r.Code!=200||r.Body.String()!="{\\\"value\\\":\\\"\\\"}\\n" {t.Fatal("another handler inherited note state")}
}
func FuzzApprovedInvariant(f *testing.F){
 f.Add([]byte{2,1,3,4,5,6});f.Add([]byte{});f.Add([]byte{255,0,8,9})
 f.Fuzz(func(t *testing.T,data []byte){
  if len(data)>96 {data=data[:96]};h:=NewHandler();saved:=""
  for index,selector:=range data {
   role:=[]string{"guest","reader","editor"}[int(selector)%3];method:=[]string{"GET","POST","DELETE"}[(int(selector)/3)%3]
   next:=fmt.Sprintf("%d:%s",index,hex.EncodeToString([]byte{selector}));bodyBytes,_:=json.Marshal(map[string]string{"value":next});body:=string(bodyBytes)
   malformed:=selector%7==0;if malformed {body="not-json"}
   want:=200
   if role=="guest" {want=403} else if method=="POST" {if role!="editor" {want=403}else if malformed {want=400}else {want=201;saved=next}} else if method!="GET" {want=405}
   got:=request(h,role,method,body);if got.Code!=want {t.Fatalf("authorization/transition mismatch at %d: got %d want %d",index,got.Code,want)}
   if want<300 {var decoded map[string]string;if json.Unmarshal(got.Body.Bytes(),&decoded)!=nil||len(decoded)!=1||decoded["value"]!=saved {t.Fatalf("round-trip state mismatch at %d",index)}}
   observed:=request(h,"reader","GET","");var decoded map[string]string
   if observed.Code!=200||json.Unmarshal(observed.Body.Bytes(),&decoded)!=nil||decoded["value"]!=saved {t.Fatalf("failed mutation changed stored state at %d",index)}
  }
 })
}
`;
const apiMain=`package main
import ("fmt"; "net/http/httptest"; "strings"; "fixture.local/workload")
func main(){h:=workload.NewHandler();write:=httptest.NewRequest("POST","/note",strings.NewReader(\u0060{"value":"approved"}\u0060));write.Header.Set("X-Role","editor");w:=httptest.NewRecorder();h.ServeHTTP(w,write);if w.Code!=201 {panic("write failed")};read:=httptest.NewRequest("GET","/note",nil);read.Header.Set("X-Role","reader");r:=httptest.NewRecorder();h.ServeHTTP(r,read);if r.Code!=200 {panic("read failed")};fmt.Print(r.Body.String())}
`;

const dataImplementation=`package workload
import ("bytes"; "encoding/csv"; "errors"; "sort"; "strconv"; "strings")
// AggregateCSV validates rows, keeps the first occurrence of each event ID,
// and emits group totals in lexical order. All accepted amounts fit the bound.
func AggregateCSV(input string)(string,error){
 reader:=csv.NewReader(strings.NewReader(input));reader.FieldsPerRecord=3
 rows,err:=reader.ReadAll();if err!=nil {return "",err}
 seen:=map[string]bool{};totals:=map[string]int64{}
 for _,row:=range rows {
  if row[0]==""||row[1]=="" {return "",errors.New("missing event identity")}
  amount,err:=strconv.ParseInt(row[2],10,64);if err!=nil||amount< -1000000||amount>1000000 {return "",errors.New("invalid amount")}
  if seen[row[0]] {continue};seen[row[0]]=true;totals[row[1]]+=amount
 }
 groups:=make([]string,0,len(totals));for group:=range totals {groups=append(groups,group)};sort.Strings(groups)
 var out bytes.Buffer;writer:=csv.NewWriter(&out)
 for _,group:=range groups {if err:=writer.Write([]string{group,strconv.FormatInt(totals[group],10)});err!=nil{return "",err}}
 writer.Flush();if err:=writer.Error();err!=nil{return "",err};return out.String(),nil
}
`;
const dataTests=`package workload
import ("bytes"; "encoding/csv"; "fmt"; "sort"; "strconv"; "testing")
func TestApprovedRequirement(t *testing.T){
 got,err:=AggregateCSV("a,red,2\\nb,blue,3\\na,red,99\\n")
 if err!=nil||got!="blue,3\\nred,2\\n" {t.Fatalf("${fixedFailure}: first-event totals/order got %q error %v",got,err)}
}
func TestMalformedInputIsRejected(t *testing.T){
 for _,input:=range []string{"a,red,nope\\n","a,red,1,extra\\n",",red,1\\n","a,,1\\n","a,red,1000001\\n","a,red,9223372036854775808\\n","\\\"unterminated"} {if _,err:=AggregateCSV(input);err==nil {t.Fatalf("accepted malformed CSV: %q",input)}}
}
// Oracle uses ordered pairwise ID comparisons and separate per-group scans,
// rather than the implementation's seen-ID and accumulated-total maps.
func dataOracle(rows [][]string)string{
 first:=make([][]string,0,len(rows))
 for i,row:=range rows {duplicate:=false;for j:=0;j<i;j++ {if rows[j][0]==row[0] {duplicate=true;break}};if !duplicate {first=append(first,row)}}
 groups:=[]string{};for _,row:=range first {exists:=false;for _,group:=range groups {if group==row[1] {exists=true}};if !exists {groups=append(groups,row[1])}};sort.Strings(groups)
 var out bytes.Buffer;writer:=csv.NewWriter(&out)
 for _,group:=range groups {total:=0;for _,row:=range first {if row[1]==group {value,_:=strconv.Atoi(row[2]);total+=value}};_ = writer.Write([]string{group,strconv.Itoa(total)})};writer.Flush();return out.String()
}
func FuzzApprovedInvariant(f *testing.F){
 f.Add([]byte{1,2,3,1,1,255,2,3,5});f.Add([]byte{});f.Add([]byte{0,0,0})
 f.Fuzz(func(t *testing.T,data []byte){
  if len(data)>96 {data=data[:96]};rows:=[][]string{}
  for i:=0;i+2<len(data);i+=3 {rows=append(rows,[]string{fmt.Sprintf("event-%d",data[i]%8),fmt.Sprintf("group-%d",data[i+1]%4),strconv.Itoa(int(int8(data[i+2])))})}
  var input bytes.Buffer;writer:=csv.NewWriter(&input);_ = writer.WriteAll(rows)
  got,err:=AggregateCSV(input.String());if err!=nil {t.Fatalf("valid generated data refused: %v",err)};if want:=dataOracle(rows);got!=want {t.Fatalf("aggregation oracle mismatch: got %q want %q",got,want)}
  again,err:=AggregateCSV(input.String());if err!=nil||again!=got {t.Fatal("aggregation was not deterministic")}
 })
}
`;
const dataMain=`package main
import ("fmt"; "fixture.local/workload")
func main(){value,err:=workload.AggregateCSV("a,red,2\\nb,blue,3\\na,red,99\\n");if err!=nil {panic(err)};fmt.Print(value)}
`;

const graphType=`type Task struct { ID string; Depends []string }\n`;
const graphImplementation=`package workload
import ("errors"; "sort")
${graphType}
// ReadyTasks validates the whole graph and returns a sorted ready frontier.
func ReadyTasks(tasks []Task,completed []string)([]string,error){
 byID:=map[string]Task{};for _,task:=range tasks {if task.ID=="" {return nil,errors.New("empty task")};if _,ok:=byID[task.ID];ok {return nil,errors.New("duplicate task")};byID[task.ID]=task}
 for _,task:=range tasks {for _,dep:=range task.Depends {if _,ok:=byID[dep];!ok{return nil,errors.New("unknown dependency")}}}
 color:=map[string]int{};var visit func(string)bool
 visit=func(id string)bool {if color[id]==1 {return false};if color[id]==2 {return true};color[id]=1;for _,dep:=range byID[id].Depends {if !visit(dep){return false}};color[id]=2;return true}
 for id:=range byID {if !visit(id){return nil,errors.New("dependency cycle")}}
 done:=map[string]bool{};for _,id:=range completed {if _,ok:=byID[id];!ok{return nil,errors.New("unknown completed task")};done[id]=true}
 ready:=[]string{};for _,task:=range tasks {if done[task.ID] {continue};allowed:=true;for _,dep:=range task.Depends {if !done[dep]{allowed=false;break}};if allowed {ready=append(ready,task.ID)}}
 sort.Strings(ready);return ready,nil
}
`;
const graphTests=`package workload
import ("fmt"; "reflect"; "sort"; "testing")
func TestApprovedRequirement(t *testing.T){
 graph:=[]Task{{ID:"review",Depends:[]string{"build"}},{ID:"plan"},{ID:"build",Depends:[]string{"plan"}},{ID:"docs"}}
 got,err:=ReadyTasks(graph,[]string{"plan"});want:=[]string{"build","docs"}
 if err!=nil||!reflect.DeepEqual(got,want) {t.Fatalf("${fixedFailure}: ready frontier got %v error %v want %v",got,err,want)}
}
func TestInvalidGraphsAreRejected(t *testing.T){
 cases:=[][]Task{{{ID:"a",Depends:[]string{"missing"}}},{{ID:"a"},{ID:"a"}},{{ID:""}},{{ID:"a",Depends:[]string{"b"}},{ID:"b",Depends:[]string{"a"}}}}
 for _,graph:=range cases {if _,err:=ReadyTasks(graph,nil);err==nil {t.Fatalf("accepted invalid graph %v",graph)}}
 if _,err:=ReadyTasks([]Task{{ID:"a"}},[]string{"missing"});err==nil {t.Fatal("accepted unknown completion")}
}
// Repeated elimination is independent of the implementation's DFS coloring.
// Input is generated as a bounded adjacency matrix with unique node IDs.
func graphOracle(edges [][]bool,completed []bool)([]string,bool){
 n:=len(edges);removed:=make([]bool,n);count:=0
 for count<n {changed:=false;for i:=0;i<n;i++ {if removed[i]{continue};incoming:=false;for j:=0;j<n;j++ {if edges[i][j]&&!removed[j] {incoming=true;break}};if !incoming {removed[i]=true;count++;changed=true}};if !changed {return nil,false}}
 ready:=[]string{};for i:=0;i<n;i++ {if completed[i]{continue};allowed:=true;for j:=0;j<n;j++ {if edges[i][j]&&!completed[j] {allowed=false}};if allowed {ready=append(ready,fmt.Sprintf("task-%d",i))}};sort.Strings(ready);return ready,true
}
func FuzzApprovedInvariant(f *testing.F){
 f.Add([]byte{3,0,1,0,2,0,4});f.Add([]byte{1,255});f.Add([]byte{})
 f.Fuzz(func(t *testing.T,data []byte){
  if len(data)>128 {data=data[:128]};if len(data)==0 {data=[]byte{0}};n:=int(data[0]%8)+1
  edges:=make([][]bool,n);done:=make([]bool,n);tasks:=make([]Task,n);completed:=[]string{}
  for i:=0;i<n;i++ {edges[i]=make([]bool,n);tasks[i].ID=fmt.Sprintf("task-%d",i);done[i]=data[(i+1)%len(data)]&1!=0;if done[i] {completed=append(completed,tasks[i].ID)};for j:=0;j<n;j++ {edges[i][j]=data[(1+i*n+j)%len(data)]&4!=0;if edges[i][j] {tasks[i].Depends=append(tasks[i].Depends,fmt.Sprintf("task-%d",j))}}}
  want,valid:=graphOracle(edges,done);got,err:=ReadyTasks(tasks,completed)
  if (err==nil)!=valid {t.Fatalf("cycle oracle disagreement: valid=%v error=%v",valid,err)}
  if valid&&!reflect.DeepEqual(got,want) {t.Fatalf("frontier oracle mismatch got %v want %v",got,want)}
 })
}
`;
const graphMain=`package main
import ("fmt"; "strings"; "fixture.local/workload")
func main(){graph:=[]workload.Task{{ID:"review",Depends:[]string{"build"}},{ID:"plan"},{ID:"build",Depends:[]string{"plan"}},{ID:"docs"}};ready,err:=workload.ReadyTasks(graph,[]string{"plan"});if err!=nil {panic(err)};fmt.Println(strings.Join(ready,","))}
`;
const definitions=[
 {label:'Escaped UI renderer',implementation:renderer(),test:rendererTests(),main:rendererMain,stub:'package workload\nfunc Render(title string,items []string)string{return ""}\n',expectedOutput:'<section><h1>Notes &amp; plans</h1><ul><li>&lt;ready&gt;</li><li>O&#39;Reilly</li></ul></section>\n',amendedImplementation:renderer(true),amendedTest:rendererTests(true),amendedExpectedOutput:'<section><h1>Notes &amp; plans</h1><ul><li>&lt;ready&gt;</li><li>O&#39;Reilly</li></ul><p>Total: 2</p></section>\n'},
 {label:'Role-scoped note HTTP API',implementation:apiImplementation,test:apiTests,main:apiMain,stub:'package workload\nimport "net/http"\nfunc NewHandler()http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){http.Error(w,"unimplemented",501)})}\n',expectedOutput:'{"value":"approved"}\n'},
 {label:'Deduplicated CSV event aggregation',implementation:dataImplementation,test:dataTests,main:dataMain,stub:'package workload\nimport "errors"\nfunc AggregateCSV(input string)(string,error){return "",errors.New("unimplemented")}\n',expectedOutput:'blue,3\nred,2\n'},
 {label:'Dependency graph ready frontier',implementation:graphImplementation,test:graphTests,main:graphMain,stub:'package workload\nimport "errors"\n'+graphType+'func ReadyTasks(tasks []Task,completed []string)([]string,error){return nil,errors.New("unimplemented")}\n',expectedOutput:'build,docs\n'},
];
export function projectWorkload(index){
 if(!Number.isInteger(index)||index<0||index>=definitions.length)throw new RangeError('Workload index must be0..3');
 const value=definitions[index];
 const result={label:value.label,redFailureMarker,fuzzTarget:'FuzzApprovedInvariant',redTest:'TestApprovedRequirement',files:{'go.mod':moduleFile,'engine.go':value.stub,'engine_test.go':value.test,'cmd/app/main.go':value.main,'witness.txt':'Approved workload '+String.fromCharCode(65+index)+': '+value.label+'\n'},implementation:value.implementation,expectedOutput:value.expectedOutput};
 if(value.amendedImplementation){result.amendedImplementation=value.amendedImplementation;result.amendedFiles={'engine_test.go':value.amendedTest,'witness.txt':'Approved workload A amendment: render exact visible item total after list\n'};result.amendedExpectedOutput=value.amendedExpectedOutput;}
 return result;
}
