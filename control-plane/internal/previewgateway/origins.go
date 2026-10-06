package previewgateway

import (
	"encoding/base32"
	"encoding/hex"
	"errors"
	"golang.org/x/net/publicsuffix"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// Public browser origins use a single DNS label so one ordinary wildcard TLS
// certificate covers them. Encoding changes only presentation: all 256 private
// capability bits survive, and existing registration files remain compatible.
var publicKeyEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func ValidatePublicAppOrigin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" || u.String() != raw || !publicDNSName(u.Hostname()) {
		return errors.New("LUCI_PUBLIC_APP_ORIGIN requires an exact HTTPS origin with a public DNS hostname and no path, credentials, query or fragment")
	}
	if u.Port() == "" && u.Host != u.Hostname() {
		return errors.New("LUCI_PUBLIC_APP_ORIGIN has a noncanonical authority")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 || port == 443 || strconv.Itoa(port) != u.Port() {
			return errors.New("LUCI_PUBLIC_APP_ORIGIN has an invalid port")
		}
	}
	return nil
}
func publicDNSName(host string) bool {
	if len(host) > 253 || host != strings.ToLower(host) || net.ParseIP(host) != nil || !strings.Contains(host, ".") {
		return false
	}
	for _, suffix := range []string{".localhost", ".local", ".internal"} {
		if strings.HasSuffix(host, suffix) {
			return false
		}
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if (ch < 'a' || ch > 'z') && (ch < '0' || ch > '9') && ch != '-' {
				return false
			}
		}
	}
	return true
}
func ValidatePreviewBaseDomain(domain string) error {
	// Reserve 55 bytes for p-<52 base32 chars> plus the separating dot.
	if !publicDNSName(domain) || len(domain) > 198 {
		return errors.New("LUCI_PREVIEW_BASE_DOMAIN requires a lowercase public DNS domain with space for an isolated project label")
	}
	return nil
}
func PublicCapabilityHostname(key, domain string) string {
	if !capability.MatchString(key) || ValidatePreviewBaseDomain(domain) != nil {
		return ""
	}
	b, _ := hex.DecodeString(key)
	return "p-" + strings.ToLower(publicKeyEncoding.EncodeToString(b)) + "." + domain
}
func publicRequestKey(host, domain string) string {
	if domain == "" || !strings.HasSuffix(host, "."+domain) {
		return ""
	}
	label := strings.TrimSuffix(host, "."+domain)
	if len(label) != 54 || !strings.HasPrefix(label, "p-") || label != strings.ToLower(label) {
		return ""
	}
	b, err := publicKeyEncoding.DecodeString(strings.ToUpper(label[2:]))
	if err != nil || len(b) != 32 {
		return ""
	}
	key := hex.EncodeToString(b)
	if PublicCapabilityHostname(key, domain) != host {
		return ""
	} // Reject alternate padding bits.
	return key
}
func browserOrigin(key string, port int, domain string) string {
	if domain != "" {
		return "https://" + PublicCapabilityHostname(key, domain)
	}
	return "http://" + CapabilityHostname(key) + ":" + strconv.Itoa(port)
}
func browserRequestKey(host string, port int, domain string) string {
	if domain != "" {
		return publicRequestKey(host, domain)
	}
	return requestKey(host, port)
}
func browserOriginKey(raw string, port int, domain string) string {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Opaque != "" || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return ""
	}
	key := browserRequestKey(u.Host, port, domain)
	if key == "" || browserOrigin(key, port, domain) != raw {
		return ""
	}
	return key
}

// AcceptsOrigin is a presentation check for a release already selected from the
// private catalogue. Management still resolves its exact team/project record
// before issuing a browser grant; this check is never authorization.
func (r *Registry) AcceptsOrigin(raw string) bool {
	return r != nil && browserOriginKey(raw, r.port, r.domain) != ""
}

// Separate registrable sites retain browser partition isolation between the
// Luci control UI and arbitrary application content, even for same-team apps.
func ValidatePublicIngress(appOrigin, domain string) error {
	if err := ValidatePublicAppOrigin(appOrigin); err != nil {
		return err
	}
	if err := ValidatePreviewBaseDomain(domain); err != nil {
		return err
	}
	u, _ := url.Parse(appOrigin)
	appSite, appErr := publicsuffix.EffectiveTLDPlusOne(u.Hostname())
	previewSite, previewErr := publicsuffix.EffectiveTLDPlusOne(domain)
	if appErr != nil || previewErr != nil || appSite == previewSite {
		return errors.New("the public app and isolated preview domain must use separate registrable DNS sites")
	}
	return nil
}
