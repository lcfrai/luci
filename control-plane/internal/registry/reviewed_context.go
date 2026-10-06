package registry

// ReviewedContext identifies the captured context delivered to this session.
// Its path is derived from the server-owned session ID and is never persisted
// here or accepted from a client. A missing descriptor on a reviewed session
// means legacy live-checkout delivery, not an implicitly reconstructable bundle.
// Keep it separate from ImplementationContext: source/review matching and
// historical run acceptance must not change when delivery is strengthened.
type ReviewedContext struct {
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

func (c *ReviewedContext) Clone() *ReviewedContext {
	if c == nil {
		return nil
	}
	copy := *c
	return &copy
}
