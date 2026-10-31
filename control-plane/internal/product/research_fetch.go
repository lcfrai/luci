package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var errResearch = errors.New("The public source could not be collected within its safety or size limits. No evidence was saved.")

func researchTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Fetch is an explicit public-source read, not search, a browser, or an agent
// tool. No cookies, ambient proxy, authentication or private-network fallback.
type ResearchCapture struct {
	ID               string `json:"id"`
	RequestedURL     string `json:"requestedUrl"`
	URL              string `json:"url"`
	Title            string `json:"title"`
	FetchedAt        string `json:"fetchedAt"`
	BodySHA256       string `json:"bodySha256"`
	TextSHA256       string `json:"textSha256"`
	Text             string `json:"text"`
	OmittedTextBytes int    `json:"omittedTextBytes"`
}
type ResearchEvidence struct {
	CaptureID        string   `json:"captureId"`
	RequestedURL     string   `json:"requestedUrl"`
	URL              string   `json:"url"`
	Title            string   `json:"title"`
	FetchedAt        string   `json:"fetchedAt"`
	BodySHA256       string   `json:"bodySha256"`
	TextSHA256       string   `json:"textSha256"`
	Quotes           []string `json:"quotes"`
	OmittedTextBytes int      `json:"omittedTextBytes"`
}

func publicResearchIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, block := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "2002::/16", "64:ff9b::/96", "64:ff9b:1::/48"} {
		if netip.MustParsePrefix(block).Contains(ip) {
			return false
		}
	}
	return true
}
func researchURL(value string) (*url.URL, error) {
	if len(value) > 2048 || strings.ContainsAny(value, "\x00\r\n\t ") {
		return nil, errResearch
	}
	u, e := url.Parse(value)
	if e != nil || u.Opaque != "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || strings.ContainsAny(u.Hostname(), "%\\") || u.Port() != "" && !(u.Scheme == "https" && u.Port() == "443" || u.Scheme == "http" && u.Port() == "80") {
		return nil, errResearch
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	for name := range u.Query() {
		switch strings.ToLower(name) {
		case "token", "access_token", "api_key", "apikey", "key", "password", "secret", "signature", "authorization", "x-amz-signature", "x-goog-signature":
			return nil, errResearch
		}
	}
	if host == "localhost" || !strings.Contains(host, ".") || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return nil, errResearch
	}
	if ip, e := netip.ParseAddr(host); e == nil && !publicResearchIP(ip) {
		return nil, errResearch
	}
	return u, nil
}
func researchDial(ctx context.Context, network, address string) (net.Conn, error) {
	return dialResearchAddress(ctx, network, address, net.DefaultResolver.LookupNetIP, (&net.Dialer{Timeout: 4 * time.Second}).DialContext)
}
func dialResearchAddress(ctx context.Context, network, address string, resolve func(context.Context, string, string) ([]netip.Addr, error), dial func(context.Context, string, string) (net.Conn, error)) (net.Conn, error) {
	host, port, e := net.SplitHostPort(address)
	if e != nil {
		return nil, errResearch
	}
	ips, e := resolve(ctx, "ip", host)
	if e != nil || len(ips) == 0 || len(ips) > 16 {
		return nil, errResearch
	}
	for _, ip := range ips {
		if !publicResearchIP(ip) {
			return nil, errResearch
		}
	}
	// One pinned address, no DNS re-resolution, retries, fallback or proxy.
	return dial(ctx, network, net.JoinHostPort(ips[0].String(), port))
}

var researchTitle = regexp.MustCompile(`(?is)<title\b[^>]*>(.*?)</title\s*>`)
var researchTags = regexp.MustCompile(`(?s)<[^>]*>`)
var researchSpace = regexp.MustCompile(`[\t\r\f ]+`)

func extractResearch(raw []byte, media string) (string, string, int, error) {
	if !utf8.Valid(raw) {
		return "", "", 0, errResearch
	}
	text, title := string(raw), "Public text source"
	if media == "text/html" {
		if match := researchTitle.FindStringSubmatch(text); len(match) == 2 {
			title = strings.Join(strings.Fields(html.UnescapeString(researchTags.ReplaceAllString(match[1], " "))), " ")
		}
		for _, tag := range []string{"script", "style", "noscript", "template", "svg", "nav", "header", "footer"} {
			text = regexp.MustCompile(`(?is)<`+tag+`\b[^>]*>.*?(?:</`+tag+`\s*>|$)`).ReplaceAllString(text, " ")
		}
		text = regexp.MustCompile(`(?i)</?(?:p|div|section|article|h[1-6]|li|br|tr)\b[^>]*>`).ReplaceAllString(text, "\n")
		text = html.UnescapeString(researchTags.ReplaceAllString(text, " "))
	}
	clean := func(s string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsControl(r) && r != '\n' || r >= 0x202a && r <= 0x202e || r >= 0x2066 && r <= 0x2069 || r == 0x200e || r == 0x200f {
				return ' '
			}
			return r
		}, s)
	}
	text, title = clean(text), clean(title)
	lines := []string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(researchSpace.ReplaceAllString(line, " "))
		if line != "" {
			lines = append(lines, line)
		}
	}
	text = strings.Join(lines, "\n")
	if text == "" {
		return "", "", 0, errResearch
	}
	original := len(text)
	if len(text) > 24<<10 {
		text = text[:24<<10]
		for !utf8.ValidString(text) {
			text = text[:len(text)-1]
		}
	}
	if strings.TrimSpace(title) == "" {
		title = "Public text source"
	}
	if len(title) > 500 {
		title = string([]rune(title)[:min(len([]rune(title)), 120)])
	}
	return title, text, original - len(text), nil
}
func fetchPublicResearch(ctx context.Context, value string) (ResearchCapture, error) {
	u, e := researchURL(value)
	if e != nil {
		return ResearchCapture{}, e
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	transport := &http.Transport{Proxy: nil, DialContext: researchDial, DisableCompression: true, DisableKeepAlives: true, TLSHandshakeTimeout: 4 * time.Second, ResponseHeaderTimeout: 6 * time.Second, MaxResponseHeaderBytes: 32 << 10, MaxConnsPerHost: 1}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 {
			return errResearch
		}
		_, e := researchURL(req.URL.String())
		return e
	}}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return ResearchCapture{}, errResearch
	}
	req.Header.Set("Accept", "text/html, text/plain")
	req.Header.Set("User-Agent", "Luci-ExplicitResearch/1.0")
	response, e := client.Do(req)
	if e != nil {
		return ResearchCapture{}, errResearch
	}
	defer response.Body.Close()
	media, params, e := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if e != nil || response.StatusCode != 200 || media != "text/html" && media != "text/plain" || params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8") || response.Header.Get("Content-Encoding") != "" && response.Header.Get("Content-Encoding") != "identity" {
		return ResearchCapture{}, errResearch
	}
	body, e := io.ReadAll(io.LimitReader(response.Body, (512<<10)+1))
	if e != nil || len(body) > 512<<10 || ctx.Err() != nil {
		return ResearchCapture{}, errResearch
	}
	title, text, omitted, e := extractResearch(body, media)
	if e != nil {
		return ResearchCapture{}, e
	}
	bodyHash, textHash := sha256.Sum256(body), sha256.Sum256([]byte(text))
	return ResearchCapture{ID: newID("research-"), RequestedURL: u.String(), URL: response.Request.URL.String(), Title: title, FetchedAt: time.Now().UTC().Format(time.RFC3339Nano), BodySHA256: hex.EncodeToString(bodyHash[:]), TextSHA256: hex.EncodeToString(textHash[:]), Text: text, OmittedTextBytes: omitted}, nil
}
