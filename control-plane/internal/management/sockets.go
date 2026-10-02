package management

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// Track the actual upgraded connection, including an inline owner's terminal.
// Cancelling a child alone cannot revoke an inline WebSocket already admitted.
type socketResponseWriter struct {
	http.ResponseWriter
	gateway             *Gateway
	userID, sessionHash string
}

func (w *socketResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *socketResponseWriter) Flush()                      { _ = http.NewResponseController(w.ResponseWriter).Flush() }
func (w *socketResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, buffer, err := http.NewResponseController(w.ResponseWriter).Hijack()
	if err != nil {
		return nil, nil, err
	}
	g := w.gateway
	g.mu.Lock()
	user, exists := g.state.Users[w.userID]
	session, valid := g.state.Sessions[w.sessionHash]
	if !exists || user.User.Disabled || !valid || session.UserID != w.userID || !time.Now().Before(session.ExpiresAt) || g.closed {
		g.mu.Unlock()
		conn.Close()
		return nil, nil, errors.New("session was revoked before upgrade")
	}
	if g.sockets[w.userID] == nil {
		g.sockets[w.userID] = map[net.Conn]string{}
	}
	if len(g.sockets[w.userID]) >= 64 {
		g.mu.Unlock()
		conn.Close()
		return nil, nil, errors.New("too many active socket connections")
	}
	g.sockets[w.userID][conn] = w.sessionHash
	g.mu.Unlock()
	wrapped := &ownedSocket{Conn: conn, close: func() { g.mu.Lock(); delete(g.sockets[w.userID], conn); g.mu.Unlock() }}
	wrapped.timer = time.AfterFunc(time.Until(session.ExpiresAt), func() { _ = conn.Close(); g.mu.Lock(); delete(g.sockets[w.userID], conn); g.mu.Unlock() })
	return wrapped, buffer, nil
}

type ownedSocket struct {
	net.Conn
	once  sync.Once
	close func()
	timer *time.Timer
}

func (c *ownedSocket) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { c.timer.Stop(); c.close() })
	return err
}
func (g *Gateway) closeSocketsLocked(userID, sessionHash string) {
	for conn, hash := range g.sockets[userID] {
		if sessionHash == "" || hash == sessionHash {
			_ = conn.Close()
			delete(g.sockets[userID], conn)
		}
	}
}
