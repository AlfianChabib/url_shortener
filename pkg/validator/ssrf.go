package validator

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
	"url_shortener/pkg/errs"
)

// Blacklisted CIDR ranges for private/local IP segments
var blacklistedIPv4CIDRs = []string{
	"127.0.0.0/8",    // IPv4 loopback
	"10.0.0.0/8",     // RFC 1918 class A private
	"172.16.0.0/12",  // RFC 1918 class B private
	"192.168.0.0/16", // RFC 1918 class C private
	"169.254.0.0/16", // IPv4 link-local (e.g., AWS/GCP metadata 169.254.169.254)
	"0.0.0.0/8",      // RFC 1122 current network
	"100.64.0.0/10",  // RFC 6598 Carrier-grade NAT
	"198.18.0.0/15",  // RFC 2544 Benchmark testing
}

var blacklistedIPv6CIDRs = []string{
	"::1/128", // IPv6 loopback
	"fc00::/7", // IPv6 Unique Local Address (ULA)
	"fe80::/10", // IPv6 Link-Local
	"::/128",    // IPv6 unspecified
}

var (
	parsedIPv4Blacklist []*net.IPNet
	parsedIPv6Blacklist []*net.IPNet
)

func init() {
	for _, cidr := range blacklistedIPv4CIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedIPv4Blacklist = append(parsedIPv4Blacklist, ipNet)
		}
	}
	for _, cidr := range blacklistedIPv6CIDRs {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			parsedIPv6Blacklist = append(parsedIPv6Blacklist, ipNet)
		}
	}
}

// DNSLookupFunc defines a function signature for resolving domain names to IP addresses.
type DNSLookupFunc func(ctx context.Context, host string) ([]net.IP, error)

// DefaultDNSLookup performs standard DNS lookup using net.DefaultResolver.
func DefaultDNSLookup(ctx context.Context, host string) ([]net.IP, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return net.DefaultResolver.LookupIP(lookupCtx, "ip", host)
}

// SafeBrowsingChecker defines the interface for checking malicious domains/URLs.
type SafeBrowsingChecker interface {
	CheckURL(ctx context.Context, rawURL string) (bool, string, error)
}

// NoopSafeBrowsingChecker is a default safe browsing checker that passes all clean URLs.
type NoopSafeBrowsingChecker struct{}

func (n *NoopSafeBrowsingChecker) CheckURL(ctx context.Context, rawURL string) (bool, string, error) {
	// By default, consider URL safe
	return true, "", nil
}

// SSRFValidator validates target URLs against SSRF, internal network leaks, and loop redirection.
type SSRFValidator interface {
	ValidateURL(ctx context.Context, rawURL string) error
	IsPrivateIP(ip net.IP) bool
}

type dnsCacheEntry struct {
	ips       []net.IP
	expiresAt time.Time
}

type ssrfValidatorImpl struct {
	dnsLookup           DNSLookupFunc
	appBaseURL          string
	safeBrowsingChecker SafeBrowsingChecker

	dnsCacheMu sync.RWMutex
	dnsCache   map[string]dnsCacheEntry
	dnsTTL     time.Duration
}

// NewSSRFValidator creates a new SSRF validator.
func NewSSRFValidator(appBaseURL string, dnsLookup DNSLookupFunc, sbChecker ...SafeBrowsingChecker) SSRFValidator {
	if dnsLookup == nil {
		dnsLookup = DefaultDNSLookup
	}
	var checker SafeBrowsingChecker = &NoopSafeBrowsingChecker{}
	if len(sbChecker) > 0 && sbChecker[0] != nil {
		checker = sbChecker[0]
	}

	return &ssrfValidatorImpl{
		dnsLookup:           dnsLookup,
		appBaseURL:          appBaseURL,
		safeBrowsingChecker: checker,
		dnsCache:            make(map[string]dnsCacheEntry),
		dnsTTL:              5 * time.Minute,
	}
}

// resolveHostWithCache resolves host IP addresses using an in-memory TTL cache to prevent DNS exhaustion under high RPS.
func (v *ssrfValidatorImpl) resolveHostWithCache(ctx context.Context, hostname string) ([]net.IP, error) {
	key := strings.ToLower(hostname)

	v.dnsCacheMu.RLock()
	entry, found := v.dnsCache[key]
	v.dnsCacheMu.RUnlock()

	if found && time.Now().Before(entry.expiresAt) {
		return entry.ips, nil
	}

	ips, err := v.dnsLookup(ctx, hostname)
	if err != nil {
		return nil, err
	}

	if len(ips) > 0 {
		v.dnsCacheMu.Lock()
		if len(v.dnsCache) >= 10000 {
			now := time.Now()
			for k, e := range v.dnsCache {
				if now.After(e.expiresAt) {
					delete(v.dnsCache, k)
				}
			}
			if len(v.dnsCache) >= 10000 {
				v.dnsCache = make(map[string]dnsCacheEntry)
			}
		}
		ttl := v.dnsTTL
		if ttl <= 0 {
			ttl = 5 * time.Minute
		}
		v.dnsCache[key] = dnsCacheEntry{
			ips:       ips,
			expiresAt: time.Now().Add(ttl),
		}
		v.dnsCacheMu.Unlock()
	}

	return ips, nil
}

// ValidateURL performs complete SSRF checks: scheme whitelisting, loop redirection prevention,
// DNS resolution, private IP blacklist checking, and safe browsing.
func (v *ssrfValidatorImpl) ValidateURL(ctx context.Context, rawURL string) error {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return errs.NewBadRequestError("URL cannot be empty")
	}

	// 1. Parse URL
	parsedURL, err := url.Parse(trimmed)
	if err != nil {
		return errs.NewBadRequestError("invalid URL format: " + err.Error())
	}

	// 2. URL Scheme Whitelisting: ONLY http and https allowed
	scheme := strings.ToLower(parsedURL.Scheme)
	if scheme != "http" && scheme != "https" {
		return errs.NewBadRequestError(fmt.Sprintf("invalid URL scheme '%s': only 'http://' and 'https://' are allowed", parsedURL.Scheme))
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return errs.NewBadRequestError("URL host cannot be empty")
	}

	// 3. Loop Redirection Protection: Do not allow shortening the shortener's own base domain
	if v.appBaseURL != "" {
		if appURL, err := url.Parse(v.appBaseURL); err == nil && appURL.Hostname() != "" {
			if strings.EqualFold(hostname, appURL.Hostname()) {
				return errs.NewBadRequestError("loop redirection is prohibited: cannot shorten URLs pointing to this service")
			}
		}
	}

	// Immediate check for common localhost literals
	lowerHost := strings.ToLower(hostname)
	if lowerHost == "localhost" || lowerHost == "127.0.0.1" || lowerHost == "::1" || lowerHost == "0.0.0.0" {
		return errs.NewBadRequestError("loopback and localhost addresses are prohibited (SSRF protection)")
	}

	// 4. DNS Resolution & Private IP Blacklisting
	// First check if host is already an IP literal
	if ip := net.ParseIP(hostname); ip != nil {
		if v.IsPrivateIP(ip) {
			return errs.NewBadRequestError(fmt.Sprintf("destination IP '%s' is in a restricted/private network segment (SSRF protection)", ip.String()))
		}
	} else {
		// Resolve domain via cached DNS lookup
		ips, err := v.resolveHostWithCache(ctx, hostname)
		if err != nil {
			return errs.NewBadRequestError(fmt.Sprintf("failed to resolve target domain '%s': %v", hostname, err))
		}
		if len(ips) == 0 {
			return errs.NewBadRequestError(fmt.Sprintf("domain '%s' did not resolve to any IP address", hostname))
		}

		for _, ip := range ips {
			if v.IsPrivateIP(ip) {
				return errs.NewBadRequestError(fmt.Sprintf("target domain '%s' resolves to a restricted/private IP address '%s' (SSRF protection)", hostname, ip.String()))
			}
		}
	}

	// 5. Safe Browsing malicious domain check
	if v.safeBrowsingChecker != nil {
		isSafe, reason, err := v.safeBrowsingChecker.CheckURL(ctx, rawURL)
		if err == nil && !isSafe {
			msg := "target URL has been flagged as unsafe"
			if reason != "" {
				msg += ": " + reason
			}
			return errs.NewBadRequestError(msg)
		}
	}

	return nil
}

// IsPrivateIP checks if the given IP address falls into loopback, private, link-local,
// carrier-grade NAT, cloud metadata, or unspecified IP ranges.
func (v *ssrfValidatorImpl) IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Normalize IPv4 or IPv4-mapped IPv6
	if ip4 := ip.To4(); ip4 != nil {
		if ip4.IsLoopback() || ip4.IsPrivate() || ip4.IsLinkLocalUnicast() || ip4.IsLinkLocalMulticast() || ip4.IsUnspecified() {
			return true
		}
		for _, network := range parsedIPv4Blacklist {
			if network.Contains(ip4) {
				return true
			}
		}
		return false
	}

	// True IPv6 checks
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return true
	}

	for _, network := range parsedIPv6Blacklist {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}
