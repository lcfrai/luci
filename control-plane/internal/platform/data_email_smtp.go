package platform

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type dataEmailSMTPResult struct{ Accepted, Uncertain bool }
type dataEmailTransport func(context.Context, dataEmailPrivateConfiguration, string) (dataEmailSMTPResult, error)
type dataEmailTLSDial func(context.Context, string) (net.Conn, error)

// GoTrue's pinned mailer always uses verified implicit TLS on port 465. Other
// ports use opportunistic STARTTLS, so this configuration deliberately accepts
// only 465 and never promises a require-STARTTLS option that GoTrue lacks.
func nativeDataEmailSMTP(ctx context.Context, c dataEmailPrivateConfiguration, recipient string) (dataEmailSMTPResult, error) {
	dialer := tls.Dialer{NetDialer: &net.Dialer{Timeout: 8 * time.Second}, Config: &tls.Config{ServerName: c.Host, MinVersion: tls.VersionTLS12}}
	return dataEmailSMTP(ctx, c, recipient, func(ctx context.Context, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", address)
	})
}

func dataEmailSMTP(ctx context.Context, c dataEmailPrivateConfiguration, recipient string, dial dataEmailTLSDial) (dataEmailSMTPResult, error) {
	var result dataEmailSMTPResult
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	conn, e := dial(ctx, net.JoinHostPort(c.Host, "465"))
	if e != nil {
		return result, &dataFailure{"The secure SMTP connection could not be verified. Check the server name, TLS certificate, port 465 and this workspace's network access."}
	}
	defer conn.Close()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	tlsConn, ok := conn.(interface{ ConnectionState() tls.ConnectionState })
	if !ok || !tlsConn.ConnectionState().HandshakeComplete || len(tlsConn.ConnectionState().VerifiedChains) == 0 {
		return result, &dataFailure{"SMTP requires a verified TLS connection. No credentials were sent."}
	}
	client, e := smtp.NewClient(conn, c.Host)
	if e != nil {
		return result, &dataFailure{"The secure server did not accept SMTP. Check the SMTP endpoint and port 465."}
	}
	defer client.Close()
	_, mechanisms := client.Extension("AUTH")
	var auth smtp.Auth
	switch {
	case strings.Contains(" "+mechanisms+" ", " CRAM-MD5 "):
		auth = smtp.CRAMMD5Auth(c.Username, c.Password)
	case strings.Contains(" "+mechanisms+" ", " PLAIN "):
		auth = smtp.PlainAuth("", c.Username, c.Password, c.Host)
	case strings.Contains(" "+mechanisms+" ", " LOGIN "):
		auth = &dataEmailLoginAuth{username: c.Username, password: c.Password}
	default:
		return result, &dataFailure{"SMTP does not advertise a supported authenticated connection. Use a provider supporting PLAIN, LOGIN or CRAM-MD5 over TLS."}
	}
	if e = client.Auth(auth); e != nil {
		return result, &dataFailure{"SMTP authentication was rejected. Check the saved username and password or provider app password."}
	}
	if recipient == "" {
		_ = client.Quit()
		return result, nil // A connection check never issues MAIL, RCPT or DATA.
	}
	if e = client.Mail(c.FromAddress); e != nil {
		return result, &dataFailure{"SMTP rejected the sender address. Verify this address with your email provider."}
	}
	if e = client.Rcpt(recipient); e != nil {
		return result, &dataFailure{"SMTP rejected the test recipient. Check the address and your provider's recipient restrictions."}
	}
	message, e := client.Data()
	if e != nil {
		return result, &dataFailure{"SMTP did not accept a test message. Check your provider's sending permissions."}
	}
	from := (&mail.Address{Name: c.SenderName, Address: c.FromAddress}).String()
	content := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: Luci application email test\r\nDate: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nThis is the test email you requested in Luci.\r\nSMTP acceptance does not confirm inbox delivery.\r\nNo account was created or changed by this test.\r\n", from, recipient, time.Now().UTC().Format(time.RFC1123Z))
	if _, e = message.Write([]byte(content)); e != nil {
		result.Uncertain = true
		return result, &dataFailure{"The test attempt was interrupted. Its acceptance could not be confirmed; check the recipient's inbox before sending another test."}
	}
	if e = message.Close(); e != nil {
		result.Uncertain = true
		return result, &dataFailure{"SMTP did not confirm the test message's acceptance. It may have been sent; check the inbox before explicitly retrying."}
	}
	result.Accepted = true
	_ = client.Quit() // Lost QUIT cannot undo a confirmed DATA response.
	return result, nil
}

type dataEmailLoginAuth struct {
	username, password string
	step               int
}

func (a *dataEmailLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, errors.New("LOGIN requires TLS")
	}
	return "LOGIN", nil, nil
}
func (a *dataEmailLoginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.username), nil
	case 2:
		return []byte(a.password), nil
	}
	return nil, errors.New("unexpected SMTP authentication challenge")
}
