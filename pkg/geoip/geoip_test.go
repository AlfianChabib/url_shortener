package geoip_test

import (
	"testing"
	"url_shortener/pkg/geoip"

	"github.com/stretchr/testify/assert"
)

func TestGeoIPResolve(t *testing.T) {
	resolver := geoip.NewResolver("ID")

	t.Run("Cloudflare Country Header takes precedence", func(t *testing.T) {
		res := resolver.Resolve("203.0.113.195", "SG")
		assert.Equal(t, "SG", res.CountryCode)
	})

	t.Run("Loopback IPv4", func(t *testing.T) {
		res := resolver.Resolve("127.0.0.1", "")
		assert.Equal(t, "ID", res.CountryCode)
		assert.Equal(t, "Localhost", res.City)
	})

	t.Run("Private RFC 1918 IP", func(t *testing.T) {
		res := resolver.Resolve("192.168.1.100", "")
		assert.Equal(t, "ID", res.CountryCode)
		assert.Equal(t, "Localhost", res.City)
	})

	t.Run("Empty IP falls back to default", func(t *testing.T) {
		res := resolver.Resolve("", "")
		assert.Equal(t, "ID", res.CountryCode)
	})
}

func TestAnonymizeIP(t *testing.T) {
	resolver := geoip.NewResolver("ID")

	hash1 := resolver.AnonymizeIP("192.168.1.1", "salt-key")
	hash2 := resolver.AnonymizeIP("192.168.1.1", "salt-key")
	hash3 := resolver.AnonymizeIP("192.168.1.2", "salt-key")

	assert.Len(t, hash1, 32)
	assert.Equal(t, hash1, hash2, "identical IP and salt should produce identical hash")
	assert.NotEqual(t, hash1, hash3, "different IP should produce different hash")
}
