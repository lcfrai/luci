package registry

// SessionStartFailure records an allocator refusal before Backend.Create was
// invoked. It is diagnostic history, never a backend absence, retirement or
// capacity-release capability. Only fixed public text is allowed here; the
// underlying error belongs in the private controller log.
type SessionStartFailure struct {
	Stage      string `json:"stage"`
	ReasonCode string `json:"reasonCode"`
	Message    string `json:"message"`
}

func (f SessionStartFailure) IsZero() bool { return f == (SessionStartFailure{}) }
func (f SessionStartFailure) Valid() bool {
	return !f.IsZero() && f == NewSessionStartFailure(f.Stage, f.ReasonCode)
}

func NewSessionStartFailure(stage, code string) SessionStartFailure {
	var message string
	switch stage {
	case "provider-reservation":
		code, message = "provider_reservation_failed", "The saved model connection could not be reserved."
	case "run-revalidation":
		code, message = "reviewed_run_changed", "The reserved project run changed before allocation."
	case "lifecycle-lock":
		code, message = "lifecycle_unavailable", "The exact worker lifecycle could not be reserved."
	case "launch-journal":
		code, message = "launch_record_unavailable", "The worker launch was retired or its durable claim could not be recorded."
	case "admission":
		message = startAdmissionMessages[code]
		if message == "" {
			code, message = "admission_unavailable", "The final execution admission could not be confirmed."
		}
	default:
		return SessionStartFailure{}
	}
	return SessionStartFailure{Stage: stage, ReasonCode: code, Message: message + " No sandbox allocation was attempted. Inspect this retained attempt before explicitly retrying."}
}

var startAdmissionMessages = map[string]string{
	"dispatch_paused":                     "New queued dispatch was paused before allocation.",
	"dispatch_policy_unavailable":         "The current dispatch policy could not be read.",
	"execution_authority_changed":         "Current execution access could not be confirmed. Review the project's access and account policy.",
	"resource_policy_changed":             "The worker resource limits changed after admission.",
	"resource_policy_unavailable":         "The current worker resource policy could not be read.",
	"admission_unavailable":               "The final execution admission could not be confirmed.",
	"resource_observation_unavailable":    "Fresh, complete machine capacity observations were unavailable at final admission.",
	"execution_limits_unknown":            "A running workload had neither verified limits nor fresh resource usage.",
	"account_session_limit":               "The account session limit was occupied at final admission.",
	"global_slot_limit":                   "The configured global agent limit was occupied at final admission.",
	"project_slot_limit":                  "The project agent limit was occupied at final admission.",
	"provider_limit":                      "The model connection concurrency limit was occupied at final admission.",
	"memory_pressure":                     "Available memory could not preserve the configured worker headroom.",
	"cpu_pressure":                        "Measured CPU pressure prevented final admission.",
	"cpu_capacity":                        "Worker CPU limits exceeded the available execution capacity.",
	"disk_observation_unavailable":        "Fresh host disk capacity was unavailable at final admission.",
	"disk_pressure":                       "Host disk capacity could not preserve the working reserve.",
	"execution_disk_unavailable":          "Fresh execution storage capacity or sandbox image demand was unavailable.",
	"execution_disk_estimate_unavailable": "The pending sandbox storage demand could not be safely estimated.",
	"execution_disk_pressure":             "Execution storage could not preserve the reserve for pending sandbox copies.",
	"host_storage_confirmation_required":  "A host administrator must confirm the active and pending local Docker storage locations.",
	"host_storage_unavailable":            "Fresh confirmed Docker backing-storage capacity was unavailable.",
	"host_storage_insufficient_capacity":  "The observed Docker backing-storage bound could not cover this worker and its reserve.",
}
