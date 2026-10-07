package platform

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type emailSMTPFixture struct {
	mu                         sync.Mutex
	commands                   []string
	messages                   []string
	listener                   net.Listener
	certificate                *x509.CertPool
	rejectAuth, loseAcceptance bool
}

func newEmailSMTPFixture(t *testing.T) *emailSMTPFixture {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "smtp.example.test"}, DNSNames: []string{"smtp.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true}
	der, e := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if e != nil {
		t.Fatal(e)
	}
	cert, e := x509.ParseCertificate(der)
	if e != nil {
		t.Fatal(e)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	listener, e := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
	if e != nil {
		t.Fatal(e)
	}
	f := &emailSMTPFixture{listener: listener, certificate: roots}
	go func() {
		for {
			conn, e := listener.Accept()
			if e != nil {
				return
			}
			go f.serve(conn)
		}
	}()
	t.Cleanup(func() { _ = listener.Close() })
	return f
}
func (f *emailSMTPFixture) dial(verify bool) dataEmailTLSDial {
	return func(ctx context.Context, address string) (net.Conn, error) {
		if address != "smtp.example.test:465" {
			return nil, &dataFailure{"Unexpected fixture address."}
		}
		config := &tls.Config{ServerName: "smtp.example.test", MinVersion: tls.VersionTLS12}
		if verify {
			config.RootCAs = f.certificate
		}
		d := tls.Dialer{Config: config}
		return d.DialContext(ctx, "tcp", f.listener.Addr().String())
	}
}
func (f *emailSMTPFixture) serve(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	write := func(v string) bool { _, e := conn.Write([]byte(v + "\r\n")); return e == nil }
	if !write("220 smtp.example.test fixture") {
		return
	}
	for {
		line, e := reader.ReadString('\n')
		if e != nil {
			return
		}
		line = strings.TrimSpace(line)
		command, argument, _ := strings.Cut(line, " ")
		f.mu.Lock()
		f.commands = append(f.commands, command)
		reject, lose := f.rejectAuth, f.loseAcceptance
		f.mu.Unlock()
		switch command {
		case "EHLO":
			if !write("250-smtp.example.test\r\n250 AUTH PLAIN LOGIN") {
				return
			}
		case "HELO":
			if !write("250 smtp.example.test") {
				return
			}
		case "AUTH":
			_, encoded, _ := strings.Cut(argument, " ")
			raw, e := base64.StdEncoding.DecodeString(encoded)
			if reject || e != nil || string(raw) != "\x00smtp-user\x00private-smtp-password" {
				if !write("535 authentication rejected") {
					return
				}
			} else if !write("235 authenticated") {
				return
			}
		case "MAIL", "RCPT":
			if !write("250 accepted") {
				return
			}
		case "DATA":
			if !write("354 write message") {
				return
			}
			var message strings.Builder
			for {
				value, e := reader.ReadString('\n')
				if e != nil {
					return
				}
				if value == ".\r\n" {
					break
				}
				message.WriteString(value)
			}
			f.mu.Lock()
			f.messages = append(f.messages, message.String())
			f.mu.Unlock()
			if lose {
				return
			}
			if !write("250 queued") {
				return
			}
		case "QUIT":
			write("221 goodbye")
			return
		case "RSET":
			if !write("250 reset") {
				return
			}
		default:
			if !write("500 unsupported") {
				return
			}
		}
	}
}
func privateEmailInput() dataEmailPrivateConfiguration {
	c := emailInput()
	return dataEmailPrivateConfiguration{Host: c.Host, Port: c.Port, Username: c.Username, Password: c.Password, FromAddress: c.FromAddress, SenderName: c.SenderName, ApplicationOrigin: c.ApplicationOrigin}
}
func TestDataEmailActualTLSCheckAuthenticatesWithoutSending(t *testing.T) {
	f := newEmailSMTPFixture(t)
	result, e := dataEmailSMTP(context.Background(), privateEmailInput(), "", f.dial(true))
	if e != nil || result.Accepted {
		t.Fatal("secure connection check failed", e)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) != 0 {
		t.Fatal("connection check sent a message")
	}
	for _, command := range f.commands {
		if command == "MAIL" || command == "RCPT" || command == "DATA" {
			t.Fatal("connection check issued sending command")
		}
	}
	if !strings.Contains(strings.Join(f.commands, " "), "AUTH") {
		t.Fatal("connection check skipped authentication")
	}
}
func TestDataEmailActualTLSRejectsUntrustedCertificateBeforeCredentials(t *testing.T) {
	f := newEmailSMTPFixture(t)
	_, e := dataEmailSMTP(context.Background(), privateEmailInput(), "", f.dial(false))
	if e == nil {
		t.Fatal("untrusted certificate accepted")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, command := range f.commands {
		if command == "AUTH" {
			t.Fatal("credential sent before trusted TLS")
		}
	}
	if strings.Contains(e.Error(), privateEmailInput().Password) {
		t.Fatal("private credential leaked")
	}
}
func TestDataEmailActualTLSMessageAcceptanceIsDistinctFromDelivery(t *testing.T) {
	f := newEmailSMTPFixture(t)
	result, e := dataEmailSMTP(context.Background(), privateEmailInput(), "recipient@example.test", f.dial(true))
	if e != nil || !result.Accepted || result.Uncertain {
		t.Fatal("SMTP acceptance not verified", e)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.messages) != 1 || !strings.Contains(f.messages[0], "acceptance does not confirm inbox delivery") || strings.Contains(f.messages[0], privateEmailInput().Password) {
		t.Fatal("test message body or privacy incorrect")
	}
}
func TestDataEmailActualTLSAuthenticationFailureAndLostReceiptAreHonest(t *testing.T) {
	t.Run("authentication", func(t *testing.T) {
		f := newEmailSMTPFixture(t)
		f.mu.Lock()
		f.rejectAuth = true
		f.mu.Unlock()
		result, e := dataEmailSMTP(context.Background(), privateEmailInput(), "recipient@example.test", f.dial(true))
		if e == nil || result.Accepted {
			t.Fatal("rejected auth claimed success")
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if len(f.messages) != 0 {
			t.Fatal("rejected auth sent a message")
		}
	})
	t.Run("lost receipt", func(t *testing.T) {
		f := newEmailSMTPFixture(t)
		f.mu.Lock()
		f.loseAcceptance = true
		f.mu.Unlock()
		result, e := dataEmailSMTP(context.Background(), privateEmailInput(), "recipient@example.test", f.dial(true))
		if e == nil || result.Accepted || !result.Uncertain {
			t.Fatal("lost DATA receipt claimed acceptance")
		}
	})
}
