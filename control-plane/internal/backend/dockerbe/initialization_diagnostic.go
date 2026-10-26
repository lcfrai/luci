package dockerbe

// initializationError preserves the existing private error and error chain.
// Public resource views use only the closed stage, never Error() or the cause.
type initializationError struct {
	stage string
	cause error
}

func (e *initializationError) Error() string                     { return e.cause.Error() }
func (e *initializationError) Unwrap() error                     { return e.cause }
func (e *initializationError) DockerInitializationStage() string { return e.stage }
