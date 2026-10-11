package e2bbe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
)

// This private file is deliberately not part of the journal/checkpoint. Loss
// permits inspect/cancel only, never implicit resume or credential reconstruction.
type guestGrant struct {
	Version            int        `json:"version"`
	OperationID        string     `json:"operationId"`
	SandboxID          string     `json:"sandboxId"`
	RequestDigest      string     `json:"requestDigest"`
	Deployment         Deployment `json:"deployment"`
	EnvdVersion        string     `json:"envdVersion"`
	EnvdAccessToken    string     `json:"envdAccessToken"`
	TrafficAccessToken string     `json:"trafficAccessToken"`
}

func (guestGrant) String() string   { return "E2B guest grant (credentials redacted)" }
func (guestGrant) GoString() string { return "E2B guest grant (credentials redacted)" }
func grantName(op string) string    { return "guest-grant-" + op + ".json" }
func (c *Controller) saveGrant(r Record, s providerSandbox) error {
	if !validToken(s.EnvdAccessToken) || !validToken(s.TrafficAccessToken) || s.EnvdVersion != EnvdVersion {
		return ErrUnsupported
	}
	grant := guestGrant{Version: 1, OperationID: r.Request.Identity.OperationID, SandboxID: r.SandboxID, RequestDigest: r.RequestDigest, Deployment: r.Deployment, EnvdVersion: s.EnvdVersion, EnvdAccessToken: s.EnvdAccessToken, TrafficAccessToken: s.TrafficAccessToken}
	b, e := json.Marshal(grant)
	if e != nil {
		return ErrUnknown
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrClosed
	}
	name := grantName(grant.OperationID)
	// The sole Create owns the sole grant publication. Never overwrite a grant
	// from an ambiguous retry, another generation, or a partial previous write.
	f, e := c.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return ErrUnknown
	}
	_, e = f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return ErrUnknown
	}
	dir, e := os.Open(c.config.StateDir)
	if e == nil {
		e = dir.Sync()
		_ = dir.Close()
	}
	if e != nil && runtime.GOOS != "windows" {
		return ErrUnknown
	}
	return nil
}
func (c *Controller) readGrant(r Record) (guestGrant, error) {
	var grant guestGrant
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return grant, ErrClosed
	}
	name := grantName(r.Request.Identity.OperationID)
	info, e := c.root.Lstat(name)
	if errors.Is(e, os.ErrNotExist) {
		return grant, ErrUnsupported
	}
	if e != nil || !info.Mode().IsRegular() || info.Size() > 16384 || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return grant, ErrConflict
	}
	f, e := c.root.Open(name)
	if e != nil {
		return grant, ErrUnknown
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 16385))
	if e != nil || len(b) > 16384 {
		return grant, ErrConflict
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&grant) != nil || d.Decode(new(any)) != io.EOF {
		return guestGrant{}, ErrConflict
	}
	if grant.Version != 1 || grant.OperationID != r.Request.Identity.OperationID || grant.SandboxID != r.SandboxID || grant.RequestDigest != r.RequestDigest || grant.Deployment != r.Deployment || grant.EnvdVersion != EnvdVersion || !validToken(grant.EnvdAccessToken) || !validToken(grant.TrafficAccessToken) {
		return guestGrant{}, ErrConflict
	}
	return grant, nil
}

// Guest obtains an exact private transport grant after current authorization.
// Restored attempts without a grant remain cancellable, but cannot be started.
func (c *Controller) Guest(ctx context.Context, op string) (*GuestTransport, error) {
	ctx, done, e := c.begin(ctx, op)
	if e != nil {
		return nil, e
	}
	defer done()
	r, e := c.record(op)
	if e != nil {
		return nil, e
	}
	if c.config.GuestOrigins == nil || r.State != Allocated || r.CancelRequested || !r.ResourcesHeld || r.Deployment != c.config.Deployment {
		return nil, ErrUnsupported
	}
	if e = c.config.Authorize(ctx, r.Request, "guest-transport"); e != nil {
		return nil, e
	}
	grant, e := c.readGrant(r)
	if e != nil {
		return nil, e
	}
	envd, sandbox, version, e := c.config.GuestOrigins(r.SandboxID)
	if e != nil || version != grant.EnvdVersion {
		return nil, ErrUnsupported
	}
	g, e := NewGuestTransport(GuestConfig{Record: r, EnvdEndpoint: envd, SandboxEndpoint: sandbox, EnvdVersion: version, EnvdAccessToken: grant.EnvdAccessToken, TrafficAccessToken: grant.TrafficAccessToken, RootCAs: c.config.RootCAs, Authorize: c.config.Authorize, CurrentRecord: func(ctx context.Context, op string) (Record, error) {
		if e := ctx.Err(); e != nil {
			return Record{}, e
		}
		c.mu.Lock()
		closed := c.closed
		c.mu.Unlock()
		if closed {
			return Record{}, ErrClosed
		}
		return c.record(op)
	}})
	if e != nil {
		return nil, e
	}
	g.owner = c
	return g, nil
}
