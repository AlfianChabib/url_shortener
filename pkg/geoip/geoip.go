package geoip

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
)

// GeoInfo represents parsed geographical metadata for an IP address.
type GeoInfo struct {
	CountryCode string `json:"country_code"`
	City        string `json:"city"`
}

// Resolver defines the GeoIP lookup interface.
type Resolver interface {
	Resolve(ip string, headerCountry string) GeoInfo
	AnonymizeIP(ip string, salt string) string
}

type geoIPResolver struct {
	defaultCountry string
}

// NewResolver creates a new GeoIP resolver with graceful fallbacks.
func NewResolver(defaultCountry ...string) Resolver {
	def := "ID"
	if len(defaultCountry) > 0 && defaultCountry[0] != "" {
		def = defaultCountry[0]
	}
	return &geoIPResolver{defaultCountry: strings.ToUpper(def)}
}

// Resolve looks up country code and city for an IP.
// It checks Cloudflare/CDN header first, handles loopback/private IPs, and falls back gracefully.
func (r *geoIPResolver) Resolve(ip string, headerCountry string) GeoInfo {
	trimmedHeader := strings.TrimSpace(headerCountry)
	if len(trimmedHeader) == 2 {
		return GeoInfo{
			CountryCode: strings.ToUpper(trimmedHeader),
			City:        "Unknown",
		}
	}

	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		return GeoInfo{
			CountryCode: r.defaultCountry,
			City:        "Unknown",
		}
	}

	parsed := net.ParseIP(cleanIP)
	if parsed == nil {
		return GeoInfo{
			CountryCode: r.defaultCountry,
			City:        "Unknown",
		}
	}

	if parsed.IsLoopback() || parsed.IsPrivate() {
		return GeoInfo{
			CountryCode: r.defaultCountry,
			City:        "Localhost",
		}
	}

	// Default fallback for external IPs
	return GeoInfo{
		CountryCode: r.defaultCountry,
		City:        "Unknown",
	}
}

// AnonymizeIP produces a 32-character hexadecimal anonymized hash of an IP address
// using HMAC-SHA256 as required by PRD Section 6.4 (Data Privacy / GDPR / UU PDP).
func (r *geoIPResolver) AnonymizeIP(ip string, salt string) string {
	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		cleanIP = "127.0.0.1"
	}

	if salt == "" {
		salt = "url-shortener-daily-salt"
	}

	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(cleanIP))
	hash := mac.Sum(nil)

	// Return 32-char hex string (first 16 bytes of HMAC-SHA256)
	return hex.EncodeToString(hash[:16])
}
