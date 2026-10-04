package validator_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"url_shortener/pkg/validator"

	"github.com/stretchr/testify/assert"
)

// mockDNSResolver provides custom DNS responses for testing
func mockDNSResolver(mockResponses map[string][]net.IP) validator.DNSLookupFunc {
	return func(ctx context.Context, host string) ([]net.IP, error) {
		if ips, ok := mockResponses[host]; ok {
			if len(ips) == 0 {
				return nil, errors.New("no such host")
			}
			return ips, nil
		}
		return []net.IP{net.ParseIP("93.184.216.34")}, nil // example.com public IP
	}
}

// mockSafeBrowsing provides configurable safe browsing results
type mockSafeBrowsing struct {
	maliciousURLs map[string]string
}

func (m *mockSafeBrowsing) CheckURL(ctx context.Context, rawURL string) (bool, string, error) {
	if reason, ok := m.maliciousURLs[rawURL]; ok {
		return false, reason, nil
	}
	return true, "", nil
}

func TestSSRFValidator_SchemeWhitelisting(t *testing.T) {
	val := validator.NewSSRFValidator("http://short.url", mockDNSResolver(nil))
	ctx := context.Background()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com", false},
		{"valid https", "https://example.com/path?foo=bar", false},
		{"ftp scheme", "ftp://example.com", true},
		{"file scheme", "file:///etc/passwd", true},
		{"data scheme", "data:text/html,<script>alert(1)</script>", true},
		{"javascript scheme", "javascript:alert(document.cookie)", true},
		{"gopher scheme", "gopher://127.0.0.1:6379", true},
		{"dict scheme", "dict://example.com", true},
		{"no scheme", "example.com", true},
		{"empty url", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := val.ValidateURL(ctx, tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSSRFValidator_LoopRedirection(t *testing.T) {
	baseDomain := "https://s.id"
	val := validator.NewSSRFValidator(baseDomain, mockDNSResolver(nil))
	ctx := context.Background()

	// Shortening shortener's own domain should be rejected
	err := val.ValidateURL(ctx, "https://s.id/something")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loop redirection is prohibited")

	err = val.ValidateURL(ctx, "http://S.ID/another-path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "loop redirection is prohibited")

	// Another domain should be accepted
	err = val.ValidateURL(ctx, "https://example.com/safe")
	assert.NoError(t, err)
}

func TestSSRFValidator_PrivateIPBlacklist(t *testing.T) {
	mockMap := map[string][]net.IP{
		"internal-service.local": {net.ParseIP("10.0.0.1")},
		"router.lan":             {net.ParseIP("192.168.1.1")},
		"staging.internal":       {net.ParseIP("172.16.10.20")},
		"metadata.cloud":         {net.ParseIP("169.254.169.254")},
		"loopback-domain.test":   {net.ParseIP("127.0.0.1")},
		"ipv6-loopback.test":     {net.ParseIP("::1")},
		"ipv6-ula.test":          {net.ParseIP("fd12:3456:789a:1::1")},
		"ipv6-link-local.test":   {net.ParseIP("fe80::1ff:fe00:3a60")},
		"cgnat.test":             {net.ParseIP("100.64.0.1")},
		"public.example.com":     {net.ParseIP("93.184.216.34")},
	}

	val := validator.NewSSRFValidator("http://short.url", mockDNSResolver(mockMap))
	ctx := context.Background()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Localhost & loopback
		{"localhost by name", "http://localhost/admin", true},
		{"localhost 127.0.0.1", "http://127.0.0.1:8080/metrics", true},
		{"loopback domain resolved", "http://loopback-domain.test", true},
		{"0.0.0.0 host", "http://0.0.0.0:3000", true},

		// RFC 1918 Private IPs
		{"10.x.x.x private IP literal", "http://10.0.0.1/status", true},
		{"10.x.x.x resolved domain", "http://internal-service.local", true},
		{"192.168.x.x private IP literal", "http://192.168.1.1/admin", true},
		{"192.168.x.x resolved domain", "http://router.lan", true},
		{"172.16.x.x private IP literal", "http://172.16.10.20", true},
		{"172.16.x.x resolved domain", "http://staging.internal", true},

		// Cloud metadata endpoint (AWS / GCP / Azure)
		{"cloud metadata literal", "http://169.254.169.254/latest/meta-data/", true},
		{"cloud metadata resolved", "http://metadata.cloud", true},

		// IPv6 restricted
		{"ipv6 loopback resolved", "http://ipv6-loopback.test", true},
		{"ipv6 ULA resolved", "http://ipv6-ula.test", true},
		{"ipv6 link-local resolved", "http://ipv6-link-local.test", true},

		// Carrier-grade NAT
		{"CGNAT resolved", "http://cgnat.test", true},

		// Public internet domain (should be allowed)
		{"public domain", "https://public.example.com/page", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := val.ValidateURL(ctx, tt.url)
			if tt.wantErr {
				assert.Error(t, err, "expected error for URL: %s", tt.url)
			} else {
				assert.NoError(t, err, "expected no error for URL: %s", tt.url)
			}
		})
	}
}

func TestSSRFValidator_SafeBrowsing(t *testing.T) {
	sb := &mockSafeBrowsing{
		maliciousURLs: map[string]string{
			"https://phishing-site.com/login": "known phishing campaign",
		},
	}

	val := validator.NewSSRFValidator("http://short.url", mockDNSResolver(nil), sb)
	ctx := context.Background()

	// Safe URL
	err := val.ValidateURL(ctx, "https://clean-site.com")
	assert.NoError(t, err)

	// Malicious URL
	err = val.ValidateURL(ctx, "https://phishing-site.com/login")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "flagged as unsafe")
	assert.Contains(t, err.Error(), "known phishing campaign")
}

func TestSSRFValidator_DNSCaching(t *testing.T) {
	callCount := 0
	countingResolver := func(ctx context.Context, host string) ([]net.IP, error) {
		callCount++
		return []net.IP{net.ParseIP("93.184.216.34")}, nil
	}

	val := validator.NewSSRFValidator("http://short.url", countingResolver)
	ctx := context.Background()

	// 1st call -> invokes resolver
	err := val.ValidateURL(ctx, "https://example.com/page1")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 2nd call to same host -> hits in-memory cache
	err = val.ValidateURL(ctx, "https://example.com/page2")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Case-insensitive host match -> hits cache
	err = val.ValidateURL(ctx, "https://EXAMPLE.COM/page3")
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}
