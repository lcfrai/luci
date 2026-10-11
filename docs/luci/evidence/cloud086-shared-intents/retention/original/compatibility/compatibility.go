package product
import "github.com/lcfrai/luci/control-plane/internal/project"
// Inert compatibility for newer shared caller sites. The original coordination
// store has no archives; these cannot supply receipts or alter the old64 bound.
func(s *Server) archivedCoordinationRequestLocked(project.Project,string,string)(*storedOrchestratorTurn,error){return nil,nil}
func(s *Server) archivedCoordinationTurnLocked(project.Project,string)(*storedOrchestratorTurn,error){return nil,nil}
func(s *Server) archivedCoordinationForeignRequestLocked(project.Project,string,string)(bool,error){return false,nil}
