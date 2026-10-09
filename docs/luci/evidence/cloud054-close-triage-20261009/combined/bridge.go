package sandboxd
import("net/http"; "github.com/lcfrai/luci/sandboxd/internal/server")
// Controlled test adapter only: expose the unchanged real server across its module internal boundary.
func ControlledServer(command string,args []string,token string)(http.Handler,func()error,error){s,e:=server.New(server.Config{Command:command,Args:args,AttachToken:token});if e!=nil{return nil,nil,e};return s,s.Close,nil}
